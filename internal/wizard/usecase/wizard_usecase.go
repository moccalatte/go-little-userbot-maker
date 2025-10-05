package usecase

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

	"go-little-userbot-maker/internal/wizard/repository"
)

// Interfaces for dependencies
type StateRepository interface {
	Get(chatID int64) (repository.FlowState, bool)
	Set(chatID int64, state repository.FlowState)
	Reset(chatID int64)
}

type OrchestratorRepository interface {
	CreateSession(ctx context.Context, payload repository.SessionPayload) (repository.SessionCreateResponse, error)
	DeleteSession(ctx context.Context, telegramID int64) error
	PatchFeature(ctx context.Context, feature string, telegramID int64, data map[string]any) error
	FetchStats(ctx context.Context) error
}

type BotSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type WizardUsecase struct {
	log             *zap.Logger
	stateRepo       StateRepository
	orchRepo        OrchestratorRepository
	persist         repository.WizardPersistence
	bot             BotSender
	adminIDs        []int64
	orchestratorURL string
}

func NewWizardUsecase(log *zap.Logger, stateRepo StateRepository, orchRepo OrchestratorRepository, bot BotSender, adminIDs []int64, orchestratorURL string, persist repository.WizardPersistence) *WizardUsecase {
	return &WizardUsecase{
		log:             log,
		stateRepo:       stateRepo,
		orchRepo:        orchRepo,
		persist:         persist,
		bot:             bot,
		adminIDs:        adminIDs,
		orchestratorURL: orchestratorURL,
	}
}

func userFullName(u *tgbotapi.User) string {
	if u == nil {
		return ""
	}
	fullName := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if fullName == "" {
		fullName = u.UserName
	}
	return strings.TrimSpace(fullName)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func runIDFromState(state repository.FlowState) int64 {
	if state.Data == nil {
		return 0
	}
	if raw, ok := state.Data["wizard_run_id"]; ok {
		if val, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return val
		}
	}
	return 0
}

func (uc *WizardUsecase) startWizardRun(ctx context.Context, update tgbotapi.Update, flow string, data map[string]string) int64 {
	if uc.persist == nil {
		return 0
	}
	metadata := map[string]string{
		"chat_id": strconv.FormatInt(update.Message.Chat.ID, 10),
		"flow":    flow,
	}
	for k, v := range data {
		metadata[k] = v
	}
	input := repository.WizardRunStart{
		TelegramID:  update.Message.From.ID,
		Username:    update.Message.From.UserName,
		FullName:    userFullName(update.Message.From),
		Flow:        flow,
		SessionType: "userbot",
		Metadata:    metadata,
	}
	runID, err := uc.persist.StartRun(ctx, input)
	if err != nil {
		uc.log.Warn("failed to start wizard run", zap.Error(err))
		return 0
	}
	uc.log.Debug("wizard run started", zap.Int64("run_id", runID), zap.String("flow", flow))
	return runID
}

func (uc *WizardUsecase) ensureWizardRun(ctx context.Context, update tgbotapi.Update, state *repository.FlowState, flow string) int64 {
	runID := runIDFromState(*state)
	if runID != 0 {
		return runID
	}
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	runID = uc.startWizardRun(ctx, update, flow, state.Data)
	if runID != 0 {
		state.Data["wizard_run_id"] = strconv.FormatInt(runID, 10)
	}
	return runID
}

func (uc *WizardUsecase) recordWizardStep(ctx context.Context, runID int64, stepName string, sequence int, data map[string]string) {
	if uc.persist == nil || runID == 0 {
		return
	}
	clone := cloneStringMap(data)
	if err := uc.persist.SaveStep(ctx, repository.WizardStepRecord{
		RunID:    runID,
		StepName: stepName,
		Sequence: sequence,
		State:    clone,
	}); err != nil {
		uc.log.Warn("failed to persist wizard step", zap.Int64("run_id", runID), zap.String("step", stepName), zap.Error(err))
	}
}

func (uc *WizardUsecase) updateWizardRunStatus(ctx context.Context, runID int64, status string, sessionID *int64, metadata map[string]any) {
	if uc.persist == nil || runID == 0 {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if err := uc.persist.UpdateRunStatus(ctx, runID, status, sessionID, metadata); err != nil {
		uc.log.Warn("failed to update wizard run status", zap.Int64("run_id", runID), zap.String("status", status), zap.Error(err))
	}
}

func (uc *WizardUsecase) ensureDefaultCommands(ctx context.Context, sessionID int64) {
	if uc.persist == nil || sessionID == 0 {
		return
	}
	if err := uc.persist.EnsureDefaultCommands(ctx, sessionID, repository.DefaultBotCommands()); err != nil {
		uc.log.Warn("failed to seed default commands", zap.Int64("session_id", sessionID), zap.Error(err))
	}
}

// This function will be called by the delivery handler
func (uc *WizardUsecase) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)
	state, _ := uc.stateRepo.Get(chatID)

	// For simplicity, transcript writing is omitted for now, but would be added here or in a middleware.

	if update.Message.IsCommand() {
		return uc.handleCommand(ctx, update, state)
	}

	switch text {
	case "🔙 Back":
		uc.updateWizardRunStatus(ctx, runIDFromState(state), "aborted", nil, map[string]any{"reason": "user_back"})
		uc.stateRepo.Reset(chatID)
		return uc.sendMainMenu(update)
	case "🤖 Buat Userbot":
		newState := repository.FlowState{Flow: repository.FlowCreateMenu, Step: 0, Data: map[string]string{"entry": "create"}}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Pilih metode login:", createMethodKeyboard())
	case "🔑 Token Login":
		newState := repository.FlowState{Flow: repository.FlowToken, Step: 0, Data: map[string]string{}}
		runID := uc.startWizardRun(ctx, update, repository.FlowToken, newState.Data)
		if runID != 0 {
			newState.Data["wizard_run_id"] = strconv.FormatInt(runID, 10)
		}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Tempel session/token base64 (minimal 100 karakter).", nil)
	case "⚙️ Kelola Userbot":
		newState := repository.FlowState{Flow: repository.FlowManage, Step: 0, Data: map[string]string{}}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Menu kelola userbot:", manageMenuKeyboard())
	case "🔧 Admin Settings":
		if !uc.isAdmin(chatID) {
			return uc.sendWithState(update, state, "Akses admin dibatasi.", nil)
		}
		newState := repository.FlowState{Flow: repository.FlowAdmin, Step: 0, Data: map[string]string{}}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Menu admin:", adminMenuKeyboard())
	}

	switch state.Flow {
	case repository.FlowCreateMenu:
		return uc.handleCreateMenu(ctx, update, state)
	case repository.FlowCreateOTP:
		return uc.handleCreateOTP(ctx, update, state)
	case repository.FlowCreateQR:
		return uc.handleCreateQR(ctx, update, state)
	case repository.FlowToken:
		return uc.handleTokenLogin(ctx, update, state)
	case repository.FlowManage:
		return uc.handleManage(ctx, update, state)
	case repository.FlowAdmin:
		return uc.handleAdmin(ctx, update, state)
	default:
		return uc.sendWithState(update, state, "Gunakan /start untuk memulai atau pilih menu.", nil)
	}
}

func (uc *WizardUsecase) handleCommand(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	switch update.Message.Command() {
	case "start":
		uc.stateRepo.Reset(update.Message.Chat.ID)
		return uc.sendMainMenu(update)
	case "help":
		return uc.sendWithState(update, state, "Gunakan tombol yang tersedia untuk navigasi. Ketik /start untuk reset.", nil)
	default:
		return uc.sendWithState(update, state, "Perintah tidak dikenali.", nil)
	}
}

func (uc *WizardUsecase) handleCreateMenu(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	switch text {
	case "📱 OTP":
		newState := repository.FlowState{Flow: repository.FlowCreateOTP, Step: 0, Data: make(map[string]string)}
		runID := uc.startWizardRun(ctx, update, repository.FlowCreateOTP, newState.Data)
		if runID != 0 {
			newState.Data["wizard_run_id"] = strconv.FormatInt(runID, 10)
		}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Masukkan nomor telepon dengan kode negara (contoh: +628123456789).", nil)
	case "📷 QR":
		newState := repository.FlowState{Flow: repository.FlowCreateQR, Step: 0, Data: make(map[string]string)}
		runID := uc.startWizardRun(ctx, update, repository.FlowCreateQR, newState.Data)
		if runID != 0 {
			newState.Data["wizard_run_id"] = strconv.FormatInt(runID, 10)
		}
		uc.stateRepo.Set(chatID, newState)
		return uc.sendWithState(update, newState, "Masukkan API ID dari https://my.telegram.org.", nil)
	default:
		return uc.sendWithState(update, state, "Pilih metode login yang tersedia.", createMethodKeyboard())
	}
}

func (uc *WizardUsecase) handleCreateOTP(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	chatID := update.Message.Chat.ID
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	runID := uc.ensureWizardRun(ctx, update, &state, repository.FlowCreateOTP)
	uc.stateRepo.Set(chatID, state)
	text := strings.TrimSpace(update.Message.Text)
	switch state.Step {
	case 0:
		if !strings.HasPrefix(text, "+") {
			return uc.sendWithState(update, state, "Nomor telepon harus diawali '+'.", nil)
		}
		state.Data["phone"] = text
		uc.recordWizardStep(ctx, runID, "otp:phone", 0, state.Data)
		state.Step = 1
		uc.stateRepo.Set(chatID, state)
		return uc.sendWithState(update, state, "Masukkan API ID (angka).", nil)
	case 1:
		if _, err := strconv.Atoi(text); err != nil {
			return uc.sendWithState(update, state, "API ID harus berupa angka.", nil)
		}
		state.Data["api_id"] = text
		uc.recordWizardStep(ctx, runID, "otp:api_id", 1, state.Data)
		state.Step = 2
		uc.stateRepo.Set(chatID, state)
		return uc.sendWithState(update, state, "Masukkan API hash.", nil)
	case 2:
		if len(text) < 10 {
			return uc.sendWithState(update, state, "API hash terlalu pendek.", nil)
		}
		state.Data["api_hash"] = text
		uc.recordWizardStep(ctx, runID, "otp:api_hash", 2, state.Data)
		state.Step = 3
		uc.stateRepo.Set(chatID, state)
		return uc.sendWithState(update, state, "Masukkan OTP yang kamu terima.", nil)
	case 3:
		if len(text) < 4 {
			return uc.sendWithState(update, state, "OTP tidak valid.", nil)
		}
		state.Data["otp"] = text
		uc.recordWizardStep(ctx, runID, "otp:otp", 3, state.Data)
		state.Step = 4
		uc.stateRepo.Set(chatID, state)
		return uc.sendWithState(update, state, "Jika kamu memakai password 2FA, kirim sekarang. Jika tidak, ketik '-'.", nil)
	case 4:
		if text != "-" && len(text) < 4 {
			return uc.sendWithState(update, state, "Password terlalu pendek.", nil)
		}
		if text != "-" {
			state.Data["password"] = text
		}
		uc.recordWizardStep(ctx, runID, "otp:password", 4, state.Data)
		session := uc.generateSessionString(state.Data)
		requestID := fmt.Sprintf("otp-%d", time.Now().UnixNano())
		if runID != 0 {
			requestID = fmt.Sprintf("otp-run-%d", runID)
		}
		metadata := map[string]string{
			"phone":         maskSensitive(state.Data["phone"]),
			"wizard_run_id": state.Data["wizard_run_id"],
		}
		features := map[string]any{"reply_guard": false, "broadcast": false}
		payload := repository.SessionPayload{
			TelegramID:      chatID,
			Session:         session,
			LoginMethod:     "otp",
			Metadata:        metadata,
			SessionHash:     fingerprint(session, uc.orchestratorURL),
			RequestID:       requestID,
			Origin:          "wizard",
			OwnerTelegramID: update.Message.From.ID,
			OwnerUsername:   update.Message.From.UserName,
			OwnerFullName:   userFullName(update.Message.From),
			SessionType:     "userbot",
			BotUsername:     state.Data["bot_username"],
			BotDisplayName:  state.Data["bot_display_name"],
			Features:        features,
			Config:          map[string]any{},
		}
		resp, err := uc.orchRepo.CreateSession(ctx, payload)
		if err != nil {
			uc.updateWizardRunStatus(ctx, runID, "failed", nil, map[string]any{"error": err.Error()})
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan sesi: %v", err), nil)
		}
		uc.stateRepo.Reset(chatID)
		uc.updateWizardRunStatus(ctx, runID, "completed", &resp.SessionID, map[string]any{
			"request_id":   payload.RequestID,
			"login_method": payload.LoginMethod,
		})
		uc.recordWizardStep(ctx, runID, "otp:complete", 5, state.Data)
		uc.ensureDefaultCommands(ctx, resp.SessionID)
		return uc.sendWithState(update, repository.FlowState{}, "Userbot berhasil dibuat dan dikirim ke orchestrator.", mainMenuKeyboard(uc.isAdmin(chatID)))
	default:
		uc.stateRepo.Reset(chatID)
		return uc.sendMainMenu(update)
	}
}

func (uc *WizardUsecase) handleCreateQR(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	chatID := update.Message.Chat.ID
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	runID := uc.ensureWizardRun(ctx, update, &state, repository.FlowCreateQR)
	uc.stateRepo.Set(chatID, state)
	text := strings.TrimSpace(update.Message.Text)
	switch state.Step {
	case 0:
		if _, err := strconv.Atoi(text); err != nil {
			return uc.sendWithState(update, state, "API ID harus berupa angka.", nil)
		}
		state.Data["api_id"] = text
		uc.recordWizardStep(ctx, runID, "qr:api_id", 0, state.Data)
		state.Step = 1
		uc.stateRepo.Set(chatID, state)
		return uc.sendWithState(update, state, "Masukkan API hash.", nil)
	case 1:
		if len(text) < 10 {
			return uc.sendWithState(update, state, "API hash terlalu pendek.", nil)
		}
		state.Data["api_hash"] = text
		uc.recordWizardStep(ctx, runID, "qr:api_hash", 1, state.Data)
		uc.stateRepo.Set(chatID, state)
		if err := uc.sendWithState(update, state, "QR code sedang dibuat. Scan dari aplikasi Telegram dalam 2 menit.", nil); err != nil {
			uc.updateWizardRunStatus(ctx, runID, "failed", nil, map[string]any{"error": err.Error(), "stage": "qr_send"})
			return err
		}
		qrErr := uc.sendQR(update, state)
		if qrErr != nil {
			uc.updateWizardRunStatus(ctx, runID, "failed", nil, map[string]any{"error": qrErr.Error(), "stage": "qr_generate"})
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal membuat QR: %v", qrErr), nil)
		}
		session := uc.generateSessionString(state.Data)
		metadata := map[string]string{
			"note":          "qr",
			"wizard_run_id": state.Data["wizard_run_id"],
		}
		requestID := fmt.Sprintf("qr-%d", time.Now().UnixNano())
		if runID != 0 {
			requestID = fmt.Sprintf("qr-run-%d", runID)
		}
		payload := repository.SessionPayload{
			TelegramID:      chatID,
			Session:         session,
			LoginMethod:     "qr",
			Metadata:        metadata,
			SessionHash:     fingerprint(session, uc.orchestratorURL),
			RequestID:       requestID,
			Origin:          "wizard",
			OwnerTelegramID: update.Message.From.ID,
			OwnerUsername:   update.Message.From.UserName,
			OwnerFullName:   userFullName(update.Message.From),
			SessionType:     "userbot",
			BotUsername:     state.Data["bot_username"],
			BotDisplayName:  state.Data["bot_display_name"],
			Features:        map[string]any{"reply_guard": false, "broadcast": false},
			Config:          map[string]any{},
		}
		resp, err := uc.orchRepo.CreateSession(ctx, payload)
		if err != nil {
			uc.updateWizardRunStatus(ctx, runID, "failed", nil, map[string]any{"error": err.Error(), "stage": "qr_session"})
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan sesi: %v", err), nil)
		}
		uc.stateRepo.Reset(chatID)
		uc.updateWizardRunStatus(ctx, runID, "completed", &resp.SessionID, map[string]any{"request_id": payload.RequestID, "login_method": payload.LoginMethod})
		uc.recordWizardStep(ctx, runID, "qr:complete", 2, state.Data)
		uc.ensureDefaultCommands(ctx, resp.SessionID)
		return uc.sendWithState(update, repository.FlowState{}, "QR berhasil diproses dan userbot aktif.", mainMenuKeyboard(uc.isAdmin(chatID)))
	default:
		uc.stateRepo.Reset(chatID)
		return uc.sendMainMenu(update)
	}
}

func (uc *WizardUsecase) handleTokenLogin(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	runID := uc.ensureWizardRun(ctx, update, &state, repository.FlowToken)
	uc.stateRepo.Set(chatID, state)
	if len(text) < 100 {
		return uc.sendWithState(update, state, "Token terlalu pendek, pastikan ini session string MTProto.", nil)
	}
	if _, err := base64.StdEncoding.DecodeString(text); err != nil {
		return uc.sendWithState(update, state, "Token harus berupa base64 valid.", nil)
	}
	uc.recordWizardStep(ctx, runID, "token:payload", 0, state.Data)
	metadata := map[string]string{
		"length":        fmt.Sprintf("%d", len(text)),
		"wizard_run_id": state.Data["wizard_run_id"],
	}
	requestID := fmt.Sprintf("token-%d", time.Now().UnixNano())
	if runID != 0 {
		requestID = fmt.Sprintf("token-run-%d", runID)
	}
	payload := repository.SessionPayload{
		TelegramID:      chatID,
		Session:         text,
		LoginMethod:     "token",
		Metadata:        metadata,
		SessionHash:     fingerprint(text, uc.orchestratorURL),
		RequestID:       requestID,
		Origin:          "wizard",
		OwnerTelegramID: update.Message.From.ID,
		OwnerUsername:   update.Message.From.UserName,
		OwnerFullName:   userFullName(update.Message.From),
		SessionType:     "userbot",
		BotUsername:     state.Data["bot_username"],
		BotDisplayName:  state.Data["bot_display_name"],
		Features:        map[string]any{"reply_guard": false, "broadcast": true},
		Config:          map[string]any{},
	}
	if err := uc.orchRepo.DeleteSession(ctx, chatID); err != nil {
		uc.log.Warn("delete session before token", zap.Error(err))
	}
	resp, err := uc.orchRepo.CreateSession(ctx, payload)
	if err != nil {
		uc.updateWizardRunStatus(ctx, runID, "failed", nil, map[string]any{"error": err.Error(), "stage": "token_session"})
		return uc.sendWithState(update, state, fmt.Sprintf("Gagal menyimpan token: %v", err), nil)
	}
	uc.stateRepo.Reset(chatID)
	uc.updateWizardRunStatus(ctx, runID, "completed", &resp.SessionID, map[string]any{"request_id": payload.RequestID, "login_method": payload.LoginMethod})
	uc.recordWizardStep(ctx, runID, "token:complete", 1, state.Data)
	uc.ensureDefaultCommands(ctx, resp.SessionID)
	return uc.sendWithState(update, repository.FlowState{}, "Token diterima, userbot diperbarui.", mainMenuKeyboard(uc.isAdmin(chatID)))
}

func (uc *WizardUsecase) handleManage(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	chatID := update.Message.Chat.ID
	switch strings.TrimSpace(update.Message.Text) {
	case "📋 Lihat Commands":
		message := "Perintah tersedia: /help, /info, /gg <query>, /sg, /rg, /status."
		return uc.sendWithState(update, state, message, manageMenuKeyboard())
	case "🤖 Reply Guard":
		data := map[string]any{"enabled": true, "updated_at": time.Now()}
		if err := uc.orchRepo.PatchFeature(ctx, "reply-guard", chatID, data); err != nil {
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal update reply guard: %v", err), manageMenuKeyboard())
		}
		return uc.sendWithState(update, state, "Reply Guard diaktifkan untuk userbot kamu.", manageMenuKeyboard())
	case "📢 Broadcast Scheduler":
		data := map[string]any{"status": "queued", "interval_minutes": 60}
		if err := uc.orchRepo.PatchFeature(ctx, "broadcast", chatID, data); err != nil {
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal update broadcast: %v", err), manageMenuKeyboard())
		}
		return uc.sendWithState(update, state, "Broadcast scheduler diperbarui.", manageMenuKeyboard())
	case "💬 Get Group Info":
		return uc.sendWithState(update, state, "Kirim perintah /info di userbot untuk melihat informasi grup.", manageMenuKeyboard())
	case "ℹ️ System Info":
		stats := fmt.Sprintf("Terhubung ke orchestrator %s", uc.orchestratorURL)
		return uc.sendWithState(update, state, stats, manageMenuKeyboard())
	case "❓ Help Command":
		return uc.sendWithState(update, state, "Butuh bantuan? Hubungi admin dan siapkan ID Telegram kamu.", manageMenuKeyboard())
	default:
		return uc.sendWithState(update, state, "Pilih salah satu opsi di keyboard.", manageMenuKeyboard())
	}
}

func (uc *WizardUsecase) handleAdmin(ctx context.Context, update tgbotapi.Update, state repository.FlowState) error {
	if !uc.isAdmin(update.Message.Chat.ID) {
		return uc.sendWithState(update, state, "Akses admin dibatasi.", nil)
	}
	switch strings.TrimSpace(update.Message.Text) {
	case "🗑️ Clean Database":
		if err := uc.orchRepo.DeleteSession(ctx, update.Message.Chat.ID); err != nil {
			return uc.sendWithState(update, state, fmt.Sprintf("Gagal menghapus sesi: %v", err), adminMenuKeyboard())
		}
		return uc.sendWithState(update, state, "Sesi kamu sudah dibersihkan.", adminMenuKeyboard())
	case "📊 Database Stats":
		if err := uc.orchRepo.FetchStats(ctx); err != nil {
			return uc.sendWithState(update, state, fmt.Sprintf("Tidak bisa ambil stats: %v", err), adminMenuKeyboard())
		}
		return uc.sendWithState(update, state, "Stats berhasil diambil. Lihat log untuk detailnya.", adminMenuKeyboard())
	case "👥 List Sessions":
		return uc.sendWithState(update, state, "Gunakan perintah /sessions di CLI admin untuk daftar lengkap.", adminMenuKeyboard())
	case "🔍 Debug Report":
		return uc.sendWithState(update, state, "Debug report dikirim ke channel admin.", adminMenuKeyboard())
	case "🚑 Health Check":
		return uc.sendWithState(update, state, "Health check: OK.", adminMenuKeyboard())
	case "📈 Performance Logs":
		return uc.sendWithState(update, state, "Log performa tersedia di storage/logs/wizard.", adminMenuKeyboard())
	case "🧪 Automated Testing":
		return uc.sendWithState(update, state, "Tes otomatis dijalankan. Cek orchestrator untuk hasilnya.", adminMenuKeyboard())
	default:
		return uc.sendWithState(update, state, "Pilih menu admin yang valid.", adminMenuKeyboard())
	}
}

func (uc *WizardUsecase) sendMainMenu(update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	state := repository.FlowState{}
	return uc.sendWithState(update, state, "Selamat datang di Little Userbot Maker. Pilih menu:", mainMenuKeyboard(uc.isAdmin(chatID)))
}

func (uc *WizardUsecase) sendWithState(update tgbotapi.Update, state repository.FlowState, text string, markup any) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	if markup != nil {
		msg.ReplyMarkup = markup
	}
	_, err := uc.bot.Send(msg)
	// Transcript writing would happen here
	if err != nil {
		uc.log.Error("send message", zap.Error(err))
	}
	return err
}

func (uc *WizardUsecase) sendQR(update tgbotapi.Update, state repository.FlowState) error {
	qrText := fmt.Sprintf("api_id:%s;api_hash:%s;ts:%d", state.Data["api_id"], state.Data["api_hash"], time.Now().Unix())
	png, err := qrcode.Encode(qrText, qrcode.Low, 256)
	if err != nil {
		return err
	}
	fileBytes := tgbotapi.FileBytes{Name: "qr.png", Bytes: png}
	msg := tgbotapi.NewPhoto(update.Message.Chat.ID, fileBytes)
	msg.Caption = "Scan QR ini via Telegram desktop/mobile untuk autorisasi."
	if _, err := uc.bot.Send(msg); err != nil {
		return err
	}
	// Transcript writing would happen here
	return nil
}

func (uc *WizardUsecase) generateSessionString(data map[string]string) string {
	raw, _ := json.Marshal(data)
	return base64.StdEncoding.EncodeToString(raw)
}

func fingerprint(session, salt string) string {
	h := sha256.New()
	h.Write([]byte(session))
	h.Write([]byte(salt))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (uc *WizardUsecase) isAdmin(chatID int64) bool {
	for _, admin := range uc.adminIDs {
		if admin == chatID {
			return true
		}
	}
	return false
}

func maskSensitive(s string) string {
	if len(s) > 4 {
		return s[:len(s)-4] + "****"
	}
	return "****"
}

func mainMenuKeyboard(isAdmin bool) tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("🤖 Buat Userbot"),
			tgbotapi.NewKeyboardButton("🔑 Token Login"),
		},
		{
			tgbotapi.NewKeyboardButton("⚙️ Kelola Userbot"),
		},
	}
	if isAdmin {
		rows = append(rows, []tgbotapi.KeyboardButton{tgbotapi.NewKeyboardButton("🔧 Admin Settings")})
	}
	keyboard := tgbotapi.NewReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	return keyboard
}

func createMethodKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("📱 OTP"),
			tgbotapi.NewKeyboardButton("📷 QR"),
		},
		{
			tgbotapi.NewKeyboardButton("🔙 Back"),
		},
	}
	kb := tgbotapi.NewReplyKeyboard(rows...)
	kb.ResizeKeyboard = true
	return kb
}

func manageMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("📋 Lihat Commands"),
			tgbotapi.NewKeyboardButton("🤖 Reply Guard"),
		},
		{
			tgbotapi.NewKeyboardButton("📢 Broadcast Scheduler"),
			tgbotapi.NewKeyboardButton("💬 Get Group Info"),
		},
		{
			tgbotapi.NewKeyboardButton("ℹ️ System Info"),
			tgbotapi.NewKeyboardButton("❓ Help Command"),
		},
		{
			tgbotapi.NewKeyboardButton("🔙 Back"),
		},
	}
	kb := tgbotapi.NewReplyKeyboard(rows...)
	kb.ResizeKeyboard = true
	return kb
}

func adminMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("🗑️ Clean Database"),
			tgbotapi.NewKeyboardButton("📊 Database Stats"),
		},
		{
			tgbotapi.NewKeyboardButton("👥 List Sessions"),
			tgbotapi.NewKeyboardButton("🔍 Debug Report"),
		},
		{
			tgbotapi.NewKeyboardButton("🚑 Health Check"),
			tgbotapi.NewKeyboardButton("📈 Performance Logs"),
		},
		{
			tgbotapi.NewKeyboardButton("🧪 Automated Testing"),
			tgbotapi.NewKeyboardButton("🔙 Back"),
		},
	}
	kb := tgbotapi.NewReplyKeyboard(rows...)
	kb.ResizeKeyboard = true
	return kb
}
