package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gary23b/easygif"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nfnt/resize"
	"github.com/shirou/gopsutil/v3/load"
)

var startTime time.Time

func main() {
	startTime = time.Now()
	godotenv.Load()

	dg, err := discordgo.New("Bot " + os.Getenv("BOT_TOKEN"))
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func checkAttachments(attachments []*discordgo.MessageAttachment) (attachs []*discordgo.MessageAttachment) {
	for _, attc := range attachments {
		if strings.HasPrefix(attc.ContentType, "image/") && attc.ContentType != "image/gif" {
			attachs = append(attachs, attc)
		}
	}

	return attachs
}

func bytesToReadable(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func downloadAndEncodeToGif(attachment *discordgo.MessageAttachment) (*bytes.Buffer, error) {
	resp, err := http.Get(attachment.URL)
	if err != nil {
		fmt.Println("Error downloading attachment:", err)
		return nil, err
	}
	defer resp.Body.Close()

	img, format, err := image.Decode(resp.Body)
	if err != nil {
		fmt.Println("Error decoding image:", err)
		return nil, err
	}

	fmt.Printf("Detected image format: %s\n", format)

	originalWidth := img.Bounds().Dx()
	originalHeight := img.Bounds().Dy()

	var newWidth, newHeight uint
	if originalWidth > originalHeight {
		newWidth = 512
		newHeight = uint(float64(originalHeight) * (float64(512) / float64(originalWidth)))
	} else {
		newHeight = 512
		newWidth = uint(float64(originalWidth) * (float64(512) / float64(originalHeight)))
	}

	resizedImg := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	images := []image.Image{resizedImg}
	gifImage := easygif.MostCommonColors(images, 0)

	buf := new(bytes.Buffer)
	err = gif.EncodeAll(buf, gifImage)
	if err != nil {
		fmt.Println("Error encoding GIF:", err)
		return nil, err
	}

	return buf, nil
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "!stats" {
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
					Name:   "CDN Stats",
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
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	}

	if m.Content == "!info" {
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

	if m.Content == "!gifify" {

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

				_, err = Upload(context.Background(), "/gifs", fileName, "", buf)
				if err != nil {
					fmt.Println("Error uploading file:", err)
					return
				}

				link := "https://pngtogif.b-cdn.net/gifs/" + fileName

				mu.Lock()
				links = append(links, link)
				mu.Unlock()
			}(a)
		}

		wg.Wait()

		s.ChannelMessageSend(m.ChannelID, strings.Join(links, "\n"))
	}
}
