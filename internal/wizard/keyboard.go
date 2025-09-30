package wizard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
