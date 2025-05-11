package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"log"
	"log/slog"
	"mehf/pngtogifbot/translations"
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

	go StartPrometheusHTTPHandler()

	commands := []*discordgo.ApplicationCommand{
		{
			Name:              "Transform images to GIF",
			NameLocalizations: translations.TransformImageToGif,
			Type:              3,
		},
		{
			Name:              "Archive existing GIF",
			NameLocalizations: translations.ArchiveGif,
			Type:              3,
		},
		{
			Name:        "stats",
			Description: "Statistics of png2gif bot",
		},
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"Transform images to GIF": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var attachments []*discordgo.MessageAttachment

			message := i.Interaction.Message

			for _, message := range i.ApplicationCommandData().Resolved.Messages {
				attachments = append(attachments, checkAttachments(message.Attachments)...)
			}

			if message != nil && len(message.Attachments) > 0 {
				attachments = append(attachments, checkAttachments(message.Attachments)...)
			}

			if len(attachments) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "No valid image attachments provided. Note that it has to be an uploaded image, not a link",
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Processing images...",
				},
			})

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

			joined := strings.Join(links, "\n")

			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &joined,
			})
		},
		"Archive existing GIF": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var attachments []*discordgo.MessageAttachment

			message := i.Interaction.Message

			for _, message := range i.ApplicationCommandData().Resolved.Messages {
				attachments = append(attachments, checkGifAttachments(message.Attachments)...)
			}

			if message != nil && len(message.Attachments) > 0 {
				attachments = append(attachments, checkGifAttachments(message.Attachments)...)
			}

			if len(attachments) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "No valid GIFs provided. Note that it has to be an uploaded gif, not a link (eg. from the favorites bar)",
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Processing images...",
				},
			})

			var links []string
			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, a := range attachments {
				wg.Add(1)

				go func(attachment *discordgo.MessageAttachment) {
					defer wg.Done()

					buf, err := downloadGif(attachment)
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

			joined := strings.Join(links, "\n")

			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &joined,
			})
		},
		"stats": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
				Title: "png2gif bot",
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
						Value: fmt.Sprintf("**Pod ID:** %s\n**Pod region:** %s\n**Pod uptime:** %s\n**CPU Load:** %s\n**RAM usage:** %d MB", os.Getenv("BUNNYNET_MC_PODID"), os.Getenv("BUNNYNET_MC_REGION"), uptimeResponse, cpuResponse, ramMB),
					},
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Pod is the current pod that is handling all Discord related stuff. ping: %dms", s.HeartbeatLatency().Milliseconds()),
				},
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:  discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{&embed},
				},
			})
		},
	}

	dg, err := discordgo.New("Bot " + os.Getenv("BOT_TOKEN"))
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	slog.Info("Checking for obsolete commands to remove...")

	commandsOnDiscord, err := dg.ApplicationCommands(dg.State.User.ID, "")
	if err != nil {
		slog.Error("failed to get Application commands")
	}
	localCommandNames := make(map[string]bool)
	for _, cmd := range commands {
		localCommandNames[cmd.Name] = true
	}

	for _, v := range commandsOnDiscord {
		if _, exists := localCommandNames[v.Name]; !exists {
			log.Printf("Deleting obsolete command: %s", v.Name)
			err := dg.ApplicationCommandDelete(dg.State.User.ID, "", v.ID)
			if err != nil {
				log.Printf("Failed to delete '%v': %v", v.Name, err)
			}
		}
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			updateMetrics(dg)
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func updateMetrics(dg *discordgo.Session) {
	botType := "png2gif"

	guilds := dg.State.Guilds
	discordGuildCount.WithLabelValues(botType).Set(float64(len(guilds)))

	var totalMembers int
	for _, guild := range guilds {
		totalMembers += len(guild.Members)
	}

	latency := dg.HeartbeatLatency().Seconds()
	discordLatency.WithLabelValues(botType).Set(latency)
}

func checkAttachments(attachments []*discordgo.MessageAttachment) (attachs []*discordgo.MessageAttachment) {
	for _, attc := range attachments {
		if strings.HasPrefix(attc.ContentType, "image/") && attc.ContentType != "image/gif" {
			attachs = append(attachs, attc)
		}
	}

	return attachs
}

func checkGifAttachments(attachments []*discordgo.MessageAttachment) (attachs []*discordgo.MessageAttachment) {
	for _, attc := range attachments {
		if attc.ContentType == "image/gif" {
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

func downloadGif(attachment *discordgo.MessageAttachment) (*bytes.Buffer, error) {
	resp, err := http.Get(attachment.URL)
	if err != nil {
		fmt.Println("Error downloading attachment:", err)
		return nil, err
	}
	defer resp.Body.Close()

	g, err := gif.DecodeAll(resp.Body)
	if err != nil {
		fmt.Println("Error decoding GIF:", err)
		return nil, err
	}

	buf := new(bytes.Buffer)

	err = gif.EncodeAll(buf, g)
	if err != nil {
		fmt.Println("Error encoding GIF:", err)
		return nil, err
	}

	return buf, nil
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

	var resizedImg image.Image

	if originalWidth <= 512 && originalHeight <= 512 {
		resizedImg = img
	} else {
		var scale float64
		if originalWidth > originalHeight {
			scale = float64(512) / float64(originalWidth)
		} else {
			scale = float64(512) / float64(originalHeight)
		}

		newWidth := uint(float64(originalWidth) * scale)
		newHeight := uint(float64(originalHeight) * scale)

		resizedImg = resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
	}

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
