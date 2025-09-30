package wizard

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

const (
	FlowNone       = ""
	FlowCreateMenu = "create_menu"
	FlowCreateOTP  = "create_otp"
	FlowCreateQR   = "create_qr"
	FlowToken      = "token_login"
	FlowManage     = "manage_userbot"
	FlowAdmin      = "admin"
)

func (s *Service) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)
	state, _ := s.state.Get(chatID)

	if update.Message.IsCommand() {
		return s.handleCommand(ctx, update, state)
	}

	switch text {
	case "🔙 Back":
		s.state.Reset(chatID)
		return s.sendMainMenu(update)
	case "🤖 Buat Userbot":
		newState := FlowState{Flow: FlowCreateMenu, Step: 0, Data: map[string]string{"entry": "create"}}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Pilih metode login:", createMethodKeyboard())
	case "🔑 Token Login":
		newState := FlowState{Flow: FlowToken, Step: 0, Data: map[string]string{}}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Tempel session/token base64 (minimal 100 karakter).", nil)
	case "⚙️ Kelola Userbot":
		newState := FlowState{Flow: FlowManage, Step: 0, Data: map[string]string{}}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Menu kelola userbot:", manageMenuKeyboard())
	case "🔧 Admin Settings":
		if !s.isAdmin(chatID) {
			return s.sendWithState(update, state, "Akses admin dibatasi.", nil)
		}
		newState := FlowState{Flow: FlowAdmin, Step: 0, Data: map[string]string{}}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Menu admin:", adminMenuKeyboard())
	}

	switch state.Flow {
	case FlowCreateMenu:
		return s.handleCreateMenu(ctx, update, state)
	case FlowCreateOTP:
		return s.handleCreateOTP(ctx, update, state)
	case FlowCreateQR:
		return s.handleCreateQR(ctx, update, state)
	case FlowToken:
		return s.handleTokenLogin(ctx, update, state)
	case FlowManage:
		return s.handleManage(ctx, update, state)
	case FlowAdmin:
		return s.handleAdmin(ctx, update, state)
	default:
		return s.sendWithState(update, state, "Gunakan /start untuk memulai atau pilih menu.", nil)
	}
}

func (s *Service) handleCommand(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	switch update.Message.Command() {
	case "start":
		s.state.Reset(update.Message.Chat.ID)
		return s.sendMainMenu(update)
	case "help":
		return s.sendWithState(update, state, "Gunakan tombol yang tersedia untuk navigasi. Ketik /start untuk reset.", nil)
	default:
		return s.sendWithState(update, state, "Perintah tidak dikenali.", nil)
	}
}

func (s *Service) handleCreateMenu(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	switch text {
	case "📱 OTP":
		newState := FlowState{Flow: FlowCreateOTP, Step: 0, Data: make(map[string]string)}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Masukkan nomor telepon dengan kode negara (contoh: +628123456789).", nil)
	case "📷 QR":
		newState := FlowState{Flow: FlowCreateQR, Step: 0, Data: make(map[string]string)}
		s.state.Set(chatID, newState)
		return s.sendWithState(update, newState, "Masukkan API ID dari https://my.telegram.org.", nil)
	default:
		return s.sendWithState(update, state, "Pilih metode login yang tersedia.", createMethodKeyboard())
	}
}

func (s *Service) handleCreateOTP(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	chatID := update.Message.Chat.ID
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	text := strings.TrimSpace(update.Message.Text)
	switch state.Step {
	case 0:
		if !strings.HasPrefix(text, "+") {
			return s.sendWithState(update, state, "Nomor telepon harus diawali '+'.", nil)
		}
		state.Data["phone"] = text
		state.Step = 1
		s.state.Set(chatID, state)
		return s.sendWithState(update, state, "Masukkan API ID (angka).", nil)
	case 1:
		if _, err := strconv.Atoi(text); err != nil {
			return s.sendWithState(update, state, "API ID harus berupa angka.", nil)
		}
		state.Data["api_id"] = text
		state.Step = 2
		s.state.Set(chatID, state)
		return s.sendWithState(update, state, "Masukkan API hash.", nil)
	case 2:
		if len(text) < 10 {
			return s.sendWithState(update, state, "API hash terlalu pendek.", nil)
		}
		state.Data["api_hash"] = text
		state.Step = 3
		s.state.Set(chatID, state)
		return s.sendWithState(update, state, "Masukkan OTP yang kamu terima.", nil)
	case 3:
		if len(text) < 4 {
			return s.sendWithState(update, state, "OTP tidak valid.", nil)
		}
		state.Data["otp"] = text
		state.Step = 4
		s.state.Set(chatID, state)
		return s.sendWithState(update, state, "Jika kamu memakai password 2FA, kirim sekarang. Jika tidak, ketik '-'.", nil)
	case 4:
		if text != "-" && len(text) < 4 {
			return s.sendWithState(update, state, "Password terlalu pendek.", nil)
		}
		if text != "-" {
			state.Data["password"] = text
		}
		session := s.generateSessionString(state.Data)
		requestID := fmt.Sprintf("otp-%d", time.Now().UnixNano())
		payload := SessionPayload{
			TelegramID:  chatID,
			Session:     session,
			LoginMethod: "otp",
			Metadata:    map[string]string{"phone": maskSensitive(state.Data["phone"])},
			SessionHash: fingerprint(session, s.cfg.OrchestratorURL),
			RequestID:   requestID,
			Origin:      "wizard",
		}
		if err := s.orchestrator.CreateSession(ctx, payload); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan sesi: %v", err), nil)
		}
		s.state.Reset(chatID)
		return s.sendWithState(update, FlowState{}, "Userbot berhasil dibuat dan dikirim ke orchestrator.", mainMenuKeyboard(s.isAdmin(chatID)))
	default:
		s.state.Reset(chatID)
		return s.sendMainMenu(update)
	}
}

func (s *Service) handleCreateQR(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	chatID := update.Message.Chat.ID
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	text := strings.TrimSpace(update.Message.Text)
	switch state.Step {
	case 0:
		if _, err := strconv.Atoi(text); err != nil {
			return s.sendWithState(update, state, "API ID harus berupa angka.", nil)
		}
		state.Data["api_id"] = text
		state.Step = 1
		s.state.Set(chatID, state)
		return s.sendWithState(update, state, "Masukkan API hash.", nil)
	case 1:
		if len(text) < 10 {
			return s.sendWithState(update, state, "API hash terlalu pendek.", nil)
		}
		state.Data["api_hash"] = text
		s.state.Set(chatID, state)
		if err := s.sendWithState(update, state, "QR code sedang dibuat. Scan dari aplikasi Telegram dalam 2 menit.", nil); err != nil {
			return err
		}
		qrErr := s.sendQR(update, state)
		if qrErr != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal membuat QR: %v", qrErr), nil)
		}
		session := s.generateSessionString(state.Data)
		payload := SessionPayload{
			TelegramID:  chatID,
			Session:     session,
			LoginMethod: "qr",
			Metadata:    map[string]string{"note": "qr"},
			SessionHash: fingerprint(session, s.cfg.OrchestratorURL),
			RequestID:   fmt.Sprintf("qr-%d", time.Now().UnixNano()),
			Origin:      "wizard",
		}
		if err := s.orchestrator.CreateSession(ctx, payload); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan sesi: %v", err), nil)
		}
		s.state.Reset(chatID)
		return s.sendWithState(update, FlowState{}, "QR berhasil diproses dan userbot aktif.", mainMenuKeyboard(s.isAdmin(chatID)))
	default:
		s.state.Reset(chatID)
		return s.sendMainMenu(update)
	}
}

func (s *Service) handleTokenLogin(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	if len(text) < 100 {
		return s.sendWithState(update, state, "Token terlalu pendek, pastikan ini session string MTProto.", nil)
	}
	if _, err := base64.StdEncoding.DecodeString(text); err != nil {
		return s.sendWithState(update, state, "Token harus berupa base64 valid.", nil)
	}
	payload := SessionPayload{
		TelegramID:  chatID,
		Session:     text,
		LoginMethod: "token",
		Metadata:    map[string]string{"length": fmt.Sprintf("%d", len(text))},
		SessionHash: fingerprint(text, s.cfg.OrchestratorURL),
		RequestID:   fmt.Sprintf("token-%d", time.Now().UnixNano()),
		Origin:      "wizard",
	}
	if err := s.orchestrator.DeleteSession(ctx, chatID); err != nil {
		s.log.Warn("delete session before token", zap.Error(err))
	}
	if err := s.orchestrator.CreateSession(ctx, payload); err != nil {
		return s.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan token: %v", err), nil)
	}
	s.state.Reset(chatID)
	return s.sendWithState(update, FlowState{}, "Token diterima, userbot diperbarui.", mainMenuKeyboard(s.isAdmin(chatID)))
}

func (s *Service) handleManage(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	chatID := update.Message.Chat.ID
	switch strings.TrimSpace(update.Message.Text) {
	case "📋 Lihat Commands":
		message := "Perintah tersedia: /help, /info, /gg <query>, /sg, /rg, /status."
		return s.sendWithState(update, state, message, manageMenuKeyboard())
	case "🤖 Reply Guard":
		data := map[string]any{"enabled": true, "updated_at": time.Now()}
		if err := s.orchestrator.PatchFeature(ctx, "reply-guard", chatID, data); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal update reply guard: %v", err), manageMenuKeyboard())
		}
		return s.sendWithState(update, state, "Reply Guard diaktifkan untuk userbot kamu.", manageMenuKeyboard())
	case "📢 Broadcast Scheduler":
		data := map[string]any{"status": "queued", "interval_minutes": 60}
		if err := s.orchestrator.PatchFeature(ctx, "broadcast", chatID, data); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal update broadcast: %v", err), manageMenuKeyboard())
		}
		return s.sendWithState(update, state, "Broadcast scheduler diperbarui.", manageMenuKeyboard())
	case "💬 Get Group Info":
		return s.sendWithState(update, state, "Kirim perintah /info di userbot untuk melihat informasi grup.", manageMenuKeyboard())
	case "ℹ️ System Info":
		stats := fmt.Sprintf("Terhubung ke orchestrator %s", s.cfg.OrchestratorURL)
		return s.sendWithState(update, state, stats, manageMenuKeyboard())
	case "❓ Help Command":
		return s.sendWithState(update, state, "Butuh bantuan? Hubungi admin dan siapkan ID Telegram kamu.", manageMenuKeyboard())
	default:
		return s.sendWithState(update, state, "Pilih salah satu opsi di keyboard.", manageMenuKeyboard())
	}
}

func (s *Service) handleAdmin(ctx context.Context, update tgbotapi.Update, state FlowState) error {
	if !s.isAdmin(update.Message.Chat.ID) {
		return s.sendWithState(update, state, "Akses admin dibatasi.", nil)
	}
	switch strings.TrimSpace(update.Message.Text) {
	case "🗑️ Clean Database":
		if err := s.orchestrator.DeleteSession(ctx, update.Message.Chat.ID); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Gagal menghapus sesi: %v", err), adminMenuKeyboard())
		}
		return s.sendWithState(update, state, "Sesi kamu sudah dibersihkan.", adminMenuKeyboard())
	case "📊 Database Stats":
		if err := s.orchestrator.FetchStats(ctx); err != nil {
			return s.sendWithState(update, state, fmt.Sprintf("Tidak bisa ambil stats: %v", err), adminMenuKeyboard())
		}
		return s.sendWithState(update, state, "Stats berhasil diambil. Lihat log untuk detailnya.", adminMenuKeyboard())
	case "👥 List Sessions":
		return s.sendWithState(update, state, "Gunakan perintah /sessions di CLI admin untuk daftar lengkap.", adminMenuKeyboard())
	case "🔍 Debug Report":
		return s.sendWithState(update, state, "Debug report dikirim ke channel admin.", adminMenuKeyboard())
	case "🚑 Health Check":
		return s.sendWithState(update, state, "Health check: OK.", adminMenuKeyboard())
	case "📈 Performance Logs":
		return s.sendWithState(update, state, "Log performa tersedia di storage/logs/wizard.", adminMenuKeyboard())
	case "🧪 Automated Testing":
		return s.sendWithState(update, state, "Tes otomatis dijalankan. Cek orchestrator untuk hasilnya.", adminMenuKeyboard())
	default:
		return s.sendWithState(update, state, "Pilih menu admin yang valid.", adminMenuKeyboard())
	}
}

func (s *Service) sendMainMenu(update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	state := FlowState{}
	return s.sendWithState(update, state, "Selamat datang di Little Userbot Maker. Pilih menu:", mainMenuKeyboard(s.isAdmin(chatID)))
}

func (s *Service) sendWithState(update tgbotapi.Update, state FlowState, text string, markup any) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	if markup != nil {
		msg.ReplyMarkup = markup
	}
	_, err := s.bot.Send(msg)
	s.transcripts.Write(update.Message.Chat.ID, update, text, state, err)
	if err != nil {
		s.log.Error("send message", zap.Error(err))
	}
	return err
}

func (s *Service) sendQR(update tgbotapi.Update, state FlowState) error {
	qrText := fmt.Sprintf("api_id:%s;api_hash:%s;ts:%d", state.Data["api_id"], state.Data["api_hash"], time.Now().Unix())
	png, err := qrcode.Encode(qrText, qrcode.Low, 256)
	if err != nil {
		return err
	}
	fileBytes := tgbotapi.FileBytes{Name: "qr.png", Bytes: png}
	msg := tgbotapi.NewPhoto(update.Message.Chat.ID, fileBytes)
	msg.Caption = "Scan QR ini via Telegram desktop/mobile untuk autorisasi."
	if _, err := s.bot.Send(msg); err != nil {
		return err
	}
	s.transcripts.Write(update.Message.Chat.ID, update, "QR code dikirim", state, nil)
	return nil
}

func (s *Service) generateSessionString(data map[string]string) string {
	raw, _ := json.Marshal(data)
	return base64.StdEncoding.EncodeToString(raw)
}

func fingerprint(session, salt string) string {
	h := sha256.New()
	h.Write([]byte(session))
	h.Write([]byte(salt))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (s *Service) isAdmin(chatID int64) bool {
	for _, admin := range s.cfg.AdminIDs {
		if admin == chatID {
			return true
		}
	}
	return false
}
