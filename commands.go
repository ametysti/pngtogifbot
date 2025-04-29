package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/load"
)

func HandlePrefixCommands(s *discordgo.Session, m *discordgo.MessageCreate) {
	prefix := "!"

	if !isRunningInDocker() {
		if m.Author.ID != "890320508984377354" {
			return
		}

		prefix = "p2g!"
	}

	if m.Content == prefix+"ping" {
		start := time.Now()
		msg, _ := s.ChannelMessageSend(m.ChannelID, "Pong!")
		latency := time.Since(start)

		embed := discordgo.MessageEmbed{
			Title: "Pong",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Roundtrip latency",
					Value:  fmt.Sprintf("%dms", latency.Milliseconds()),
					Inline: true,
				},
				{
					Name:   "Discord Latency",
					Value:  fmt.Sprintf("%dms", s.HeartbeatLatency().Milliseconds()),
					Inline: true,
				},
			},
		}

		s.ChannelMessageEditEmbed(m.ChannelID, msg.ID, &embed)
	}

	if m.Content == prefix+"stats" {
		cdn, cdnErr := GetPullZoneStats()
		storage, storageErr := GetStorageZoneStats()
		uptime := time.Since(startTime)

		days := int(uptime.Hours()) / 24
		hours := int(uptime.Hours()) % 24
		minutes := int(uptime.Minutes()) % 60
		seconds := int(uptime.Seconds()) % 60

		uptimeResponse := fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)

		var mstats runtime.MemStats
		runtime.ReadMemStats(&mstats)

		ramMB := mstats.Alloc / 1024 / 1024

		avg, _ := load.Avg()
		cpuResponse := fmt.Sprintf("%.2f %.2f %.2f", avg.Load1, avg.Load5, avg.Load15)

		cdnResponse := fmt.Sprintf("**Bandwidth:** %s\n**Requests:** %d requests", bytesToReadable(cdn.TotalBandwidthUsed), cdn.TotalRequestsServed)
		storageResponse := fmt.Sprintf("**Storage used:** %s\n**Files stored:** %d files", bytesToReadable(int64(storage.StorageUsed)), storage.FileCount)

		if cdnErr != nil {
			cdnResponse = "CDN Stats API Down"
		}

		if storageErr != nil {
			storageResponse = "Storage Stats API Down"
		}

		embed := discordgo.MessageEmbed{
			Title: "png to gif bot",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "CDN Stats past month",
					Value:  cdnResponse,
					Inline: true,
				},
				{
					Name:   "Storage Stats",
					Value:  storageResponse,
					Inline: true,
				},
				{
					Name:  "Bot stats",
					Value: fmt.Sprintf("**Uptime:** %s\n**CPU Load:** %s\n**RAM usage:** %d MB", uptimeResponse, cpuResponse, ramMB),
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("%dms", s.HeartbeatLatency().Milliseconds()),
			},
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	}

	if m.Content == prefix+"info" {
		embed := discordgo.MessageEmbed{
			Title: "png to gif bot",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "what is this bot",
					Value: "This bot allows you to transform an image into a GIF directly from Discord. Images are uploaded to datacenters in both Europe and the USA to ensure high availability.",
				},
				{
					Name:  "Why this bot?",
					Value: "If you upload a GIF directly to Discord and save it to your favorites, it will eventually expire because Discord uses token-based authentication for images.",
				},
			},
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &embed)

	}

	if m.Content == prefix+"gifify" {

		var attachments []*discordgo.MessageAttachment

		if m.ReferencedMessage != nil && len(m.ReferencedMessage.Attachments) > 0 {
			attachments = append(attachments, checkAttachments(m.ReferencedMessage.Attachments)...)
		}

		if len(m.Attachments) > 0 {
			attachments = append(attachments, checkAttachments(m.Attachments)...)
		}

		if len(attachments) == 0 {
			s.ChannelMessageSend(m.ChannelID, "no valid image attachments provided. note that this system only allows uploading static images.")
			return
		}

		s.ChannelTyping(m.ChannelID)

		var links []string

		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, a := range attachments {
			wg.Add(1)

			go func(attachment *discordgo.MessageAttachment) {
				defer wg.Done()

				buf, err := downloadAndEncodeToGif(attachment)
				if err != nil {
					fmt.Println("error processing attachment:", err)
					return
				}

				name := uuid.New()
				fileName := fmt.Sprintf("%s.gif", name)

				folder := "/gifs"

				if !isRunningInDocker() {
					folder = "/dev/gifs"
				}

				_, err = Upload(context.Background(), folder, fileName, "", buf)
				if err != nil {
					fmt.Println("Error uploading file:", err)
					return
				}

				link := "https://pngtogif.b-cdn.net" + folder + "/" + fileName

				mu.Lock()
				links = append(links, link)
				mu.Unlock()
			}(a)
		}

		wg.Wait()

		s.ChannelMessageSend(m.ChannelID, strings.Join(links, "\n"))
	}
}
