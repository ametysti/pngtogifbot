package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"io"
	"log/slog"
	"mehf/pngtogifbot/translations"
	"net/http"
	"os"
	"os/exec"
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
	"github.com/shirou/gopsutil/v3/load"
	Reporter "github.com/valeriansaliou/go-vigil-reporter/vigil_reporter"
)

var startTime time.Time

func main() {
	startTime = time.Now()
	godotenv.Load()

	if isRunningInDocker() {
		builder := Reporter.New(os.Getenv("VIGIL_REPORTER_URL"), os.Getenv("VIGIL_REPORTER_TOKEN"))
		reporter := builder.ProbeID("png2gif").NodeID("png2gif-bot").ReplicaID(fmt.Sprintf("%s-%s", os.Getenv("BUNNYNET_MC_REGION"), os.Getenv("BUNNYNET_MC_PODID"))).Interval(time.Duration(30 * time.Second)).Build()
		reporter.Run()
	}

	go StartPrometheusHTTPHandler()

	commands := []*discordgo.ApplicationCommand{
		{
			Name:              "Archive existing GIF",
			NameLocalizations: translations.ArchiveGif,
			Type:              3,
		},
		{
			Name: "Transform files to GIFs",
			Type: 3,
		},
		{
			Name:        "stats",
			Description: "Statistics of png2gif bot",
		},
	}

	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"Transform files to GIFs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			var attachments []*discordgo.MessageAttachment

			message := i.Interaction.Message

			for _, message := range i.ApplicationCommandData().Resolved.Messages {
				attachments = append(attachments, checkAttachments(message.Attachments, "image/")...)
				attachments = append(attachments, checkAttachments(message.Attachments, "video/")...)
			}

			if message != nil && len(message.Attachments) > 0 {
				attachments = append(attachments, checkAttachments(message.Attachments, "image/")...)
				attachments = append(attachments, checkAttachments(message.Attachments, "video/")...)

			}

			if len(attachments) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "No valid file (image or video) attachments provided.",
					},
				})
				return
			}

			processingMsg := "Processing file..."

			if len(attachments) > 1 {
				processingMsg = "Processing files..."
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: processingMsg,
				},
			})

			var links []string
			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, a := range attachments {
				wg.Add(1)

				go func(attachment *discordgo.MessageAttachment) {
					defer wg.Done()

					var buf *bytes.Buffer
					var err error

					if strings.HasPrefix(attachment.ContentType, "image/") {
						buf, err = downloadAndEncodeToGif(attachment)
					}

					if strings.HasPrefix(attachment.ContentType, "video/") {
						buf, err = downloadVideoAndEncodeToGif(attachment)
					}

					if err != nil {
						fmt.Println("error processing attachment:", err)

						msg := "Failed to process videos"

						s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
							Content: &msg,
						})

						return
					}

					name := uuid.New()
					fileName := fmt.Sprintf("%s.gif", name)

					if !isRunningInDocker() {
						fileName = fmt.Sprintf("%s_devenv.gif", name)
					}

					_, err = Upload(context.Background(), "/gifs", fileName, "", buf)
					if err != nil {
						fmt.Println("Error uploading file:", err)
						return
					}

					link := "https://p2gcdn.netstat.ovh" + "/gifs" + "/" + fileName

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
				attachments = append(attachments, checkAttachments(message.Attachments, "image/gif")...)
			}

			if message != nil && len(message.Attachments) > 0 {
				attachments = append(attachments, checkAttachments(message.Attachments, "image/gif")...)
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

					if !isRunningInDocker() {
						fileName = fmt.Sprintf("%s_devenv.gif", name)
					}

					_, err = Upload(context.Background(), "/gifs", fileName, "", buf)
					if err != nil {
						fmt.Println("Error uploading file:", err)
						return
					}

					link := "https://p2gcdn.netstat.ovh" + "/gifs" + "/" + fileName

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
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Fetching statistics data...",
				},
			})

			cdn, cdnErr := GetPullZoneStats()
			totalFiles, totalSize, storageErr := GetStorageZoneStats()
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
			storageResponse := fmt.Sprintf("**Storage used:** %s\n**Files stored:** %d files", bytesToReadable(totalSize), totalFiles)

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
						Value: fmt.Sprintf("**Uptime:** %s\n**CPU Load:** %s\n**RAM usage:** %d MB", uptimeResponse, cpuResponse, ramMB),
					},
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("%s-%s | Ping: %dms", os.Getenv("BUNNYNET_MC_REGION"), os.Getenv("BUNNYNET_MC_PODID"), s.HeartbeatLatency().Milliseconds()),
				},
			}

			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Embeds: &[]*discordgo.MessageEmbed{&embed},
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

	slog.Info("[DISCORD] Creating websocket connection")

	err = dg.Open()
	if err != nil {
		slog.Error("[DISCORD] Failed to create websocket connection", "error", err)
		return
	} else {
		slog.Info("[DISCORD] Created websocket connection successfully")
	}

	slog.Info("[DISCORD] Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			slog.Error("[DISCORD] Cannot create command", "command", v.Name, "error", err)
		}
		registeredCommands[i] = cmd
	}

	slog.Info("[DISCORD] Checking for obsolete commands to remove...")

	commandsOnDiscord, err := dg.ApplicationCommands(dg.State.User.ID, "")
	if err != nil {
		slog.Error("[DISCORD] failed to get Application commands")
	}
	localCommandNames := make(map[string]bool)
	for _, cmd := range commands {
		localCommandNames[cmd.Name] = true
	}

	for _, v := range commandsOnDiscord {
		if _, exists := localCommandNames[v.Name]; !exists {
			slog.Info("[DISCORD] Deleting obsolete command.", "name", v.Name)
			err := dg.ApplicationCommandDelete(dg.State.User.ID, "", v.ID)
			if err != nil {
				slog.Error("[DISCORD] Failed to delete obsolete command.", "name", v.Name, "error", err)
			}
		}
	}

	slog.Info("[DISCORD] Bot is now running.")

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

func checkAttachments(attachments []*discordgo.MessageAttachment, contentTypePrefix string) (attachs []*discordgo.MessageAttachment) {
	if contentTypePrefix == "" {
		contentTypePrefix = "image/"
	}

	for _, attc := range attachments {
		if strings.HasPrefix(attc.ContentType, contentTypePrefix) {
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

	images := []image.Image{img}
	gifImage := easygif.MostCommonColors(images, 0)

	buf := new(bytes.Buffer)
	err = gif.EncodeAll(buf, gifImage)
	if err != nil {
		fmt.Println("Error encoding GIF:", err)
		return nil, err
	}

	return buf, nil
}

func downloadVideoAndEncodeToGif(attachment *discordgo.MessageAttachment) (*bytes.Buffer, error) {
	resp, err := http.Get(attachment.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer resp.Body.Close()

	tmpIn, err := os.CreateTemp("", "input-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(tmpIn.Name())

	_, err = io.Copy(tmpIn, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write to temp input file: %w", err)
	}

	tmpIn.Close()

	tmpOut, err := os.CreateTemp("", "output-*.gif")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	defer os.Remove(tmpOut.Name())
	tmpOut.Close()

	tmpPalette, err := os.CreateTemp("", "palette-*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp palette file: %w", err)
	}
	defer os.Remove(tmpPalette.Name())

	ffmpegLocation := "bin/ffmpeg-win/ffmpeg.exe"

	if isRunningInDocker() {
		ffmpegLocation = "ffmpeg"
	}

	cmd1 := exec.Command(ffmpegLocation,
		"-t", "10",
		"-i", tmpIn.Name(),
		"-vf", "fps=12,scale=480:-1:flags=lanczos,palettegen",
		"-y",
		tmpPalette.Name(),
	)

	var stderr1 bytes.Buffer
	cmd1.Stderr = &stderr1

	if err := cmd1.Run(); err != nil {
		return nil, fmt.Errorf("failed to generate palette: %v\n%s", err, stderr1.String())
	}

	cmd2 := exec.Command(ffmpegLocation,
		"-t", "10",
		"-i", tmpIn.Name(),
		"-i", tmpPalette.Name(),
		//"-lavfi", "fps=12,scale=480:-1:flags=lanczos [x]; [x][1:v] paletteuse=dither=sierra2_4a",
		"-lavfi", "fps=8,scale=480:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3",
		"-y",
		tmpOut.Name(),
	)

	var stderr2 bytes.Buffer
	cmd2.Stderr = &stderr2

	if err := cmd2.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg error: %v\n%s", err, stderr2.String())
	}

	outBytes, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	return bytes.NewBuffer(outBytes), nil
}
