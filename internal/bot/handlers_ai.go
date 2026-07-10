// Package bot — handlers_ai.go — AI-powered commands via OnlySQ API.
package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/pweper/bot/internal/ai"
	"github.com/pweper/bot/internal/bot/middleware"
	"github.com/pweper/bot/internal/timecyc"
)

// getOnlySQClient returns an OnlySQ client if API key is configured.
func (b *Bot) getOnlySQClient() *ai.OnlySQClient {
	if b.cfg.OnlySQAPIKey == "" {
		return nil
	}
	return ai.NewOnlySQClient(b.cfg.OnlySQAPIKey, "")
}

// handleAITimecyc — generates timecyc.json from a text description.
func (b *Bot) handleAITimecyc(c *middleware.Ctx, parts []string) {
	client := b.getOnlySQClient()
	if client == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❌ AI не настроен. Обратитесь к администратору."))
		return
	}
	if len(parts) < 2 {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❔ Формат: /aitimecyc <описание>\nПример: /aitimecyc закат с алыми облаками"))
		return
	}
	description := strings.Join(parts[1:], " ")
	progress, _ := b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "🤖 Генерирую таймсайк..."))

	ctx, cancel := context.WithTimeout(c, 60*time.Second)
	defer cancel()

	colors, err := client.GenerateTimecycColors(ctx, description)
	if err != nil {
		b.editText(c, progress, "❌ Ошибка AI: "+err.Error())
		return
	}

	jsonStr := timecyc.Generate(timecyc.Colors{
		SkyBottomRGB: colors["SkyBottomRGB"],
		SkyTopRGB:    colors["SkyTopRGB"],
		CloudRGB:     colors["CloudRGB"],
		SunCoreRGB:   colors["SunCoreRGB"],
	})

	b.deleteMsg(c, progress)
	doc := tgbotapi.NewDocument(c.Message.Chat.ID, tgbotapi.FileReader{
		Reader: strings.NewReader(jsonStr),
		Name:   "aitimecyc.json",
	})
	doc.Caption = fmt.Sprintf("🎨 <b>AI TimeCyc</b>\n<i>%s</i>", description)
	doc.ParseMode = "HTML"
	_, _ = b.api.Send(doc)
}

// handleAIColor — generates a hex color from a text description.
func (b *Bot) handleAIColor(c *middleware.Ctx, parts []string) {
	client := b.getOnlySQClient()
	if client == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❌ AI не настроен. Обратитесь к администратору."))
		return
	}
	if len(parts) < 2 {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❔ Пример: /aicolor свет от луны"))
		return
	}
	description := strings.Join(parts[1:], " ")
	progress, _ := b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "🤖 Подбираю цвет..."))

	ctx, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()

	hexColor, err := client.GenerateColor(ctx, description)
	if err != nil {
		b.editText(c, progress, "❌ Ошибка AI: "+err.Error())
		return
	}

	b.deleteMsg(c, progress)
	_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
		fmt.Sprintf("🎨 <b>AI цвет для «%s»</b>\n\nHex: <code>%s</code>", description, hexColor)))
}

// handleImgDescribe — describes an image using AI vision.
func (b *Bot) handleImgDescribe(c *middleware.Ctx) {
	client := b.getOnlySQClient()
	if client == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❌ AI не настроен. Обратитесь к администратору."))
		return
	}
	if c.Message.Document == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"Отправьте изображение с подписью /imgdescribe"))
		return
	}
	doc := c.Message.Document
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "❌ "+err.Error()))
		return
	}
	imgData, err := httpGet(file.Link(b.api.Token))
	if err != nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "❌ "+err.Error()))
		return
	}
	progress, _ := b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "🤖 Анализирую изображение..."))

	ctx, cancel := context.WithTimeout(c, 60*time.Second)
	defer cancel()

	// Determine format
	format := "png"
	if strings.HasSuffix(strings.ToLower(doc.FileName), ".jpg") ||
		strings.HasSuffix(strings.ToLower(doc.FileName), ".jpeg") {
		format = "jpeg"
	}

	description, err := client.DescribeImage(ctx, imgData, format)
	if err != nil {
		b.editText(c, progress, "❌ Ошибка AI: "+err.Error())
		return
	}

	b.deleteMsg(c, progress)
	_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
		fmt.Sprintf("📝 <b>Описание изображения:</b>\n\n%s", description)))
}

// handleImg2Prompt — generates a Stable Diffusion prompt from an image.
func (b *Bot) handleImg2Prompt(c *middleware.Ctx) {
	client := b.getOnlySQClient()
	if client == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"❌ AI не настроен. Обратитесь к администратору."))
		return
	}
	if c.Message.Document == nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
			"Отправьте изображение с подписью /img2prompt"))
		return
	}
	doc := c.Message.Document
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "❌ "+err.Error()))
		return
	}
	imgData, err := httpGet(file.Link(b.api.Token))
	if err != nil {
		_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "❌ "+err.Error()))
		return
	}
	progress, _ := b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID, "🤖 Генерирую промпт..."))

	ctx, cancel := context.WithTimeout(c, 60*time.Second)
	defer cancel()

	format := "png"
	if strings.HasSuffix(strings.ToLower(doc.FileName), ".jpg") ||
		strings.HasSuffix(strings.ToLower(doc.FileName), ".jpeg") {
		format = "jpeg"
	}

	prompt, err := client.ImageToPrompt(ctx, imgData, format)
	if err != nil {
		b.editText(c, progress, "❌ Ошибка AI: "+err.Error())
		return
	}

	b.deleteMsg(c, progress)
	_, _ = b.api.Send(tgbotapi.NewMessage(c.Message.Chat.ID,
		fmt.Sprintf("🎨 <b>Stable Diffusion промпт:</b>\n\n<code>%s</code>", prompt)))
}

// ── helpers ────────────────────────────────────────────────────────────────

func (b *Bot) editText(c *middleware.Ctx, msg tgbotapi.Message, text string) {
	if msg.Chat == nil {
		return
	}
	edit := tgbotapi.NewEditMessageText(c.Message.Chat.ID, msg.MessageID, text)
	_, _ = b.api.Send(edit)
}

func (b *Bot) deleteMsg(c *middleware.Ctx, msg tgbotapi.Message) {
	if msg.Chat == nil {
		return
	}
	_, _ = b.api.Request(tgbotapi.NewDeleteMessage(c.Message.Chat.ID, msg.MessageID))
}

// Ensure imports are used
var _ = io.ReadAll
var _ = bytes.NewReader
var _ = middleware.Ctx{}
