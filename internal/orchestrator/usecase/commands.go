package usecase

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Command interface {
	Name() string
	Help() string
	Execute(ctx context.Context, payload CommandPayload) error
}

type CommandPayload struct {
	TelegramID int64
	Args       []string
	Metadata   map[string]string
}

type Registry struct {
	log      *zap.Logger
	mu       sync.RWMutex
	commands map[string]Command
}

func NewRegistry(log *zap.Logger) *Registry {
	r := &Registry{log: log, commands: make(map[string]Command)}
	r.Register(&HelpCommand{log: log, registry: r})
	r.Register(&InfoCommand{log: log})
	r.Register(&StatusCommand{log: log})
	r.Register(&GoogleCommand{log: log})
	r.Register(&ReplyGuardCommand{log: log})
	r.Register(&BroadcastCommand{log: log})
	return r
}

func (r *Registry) Register(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[cmd.Name()] = cmd
}

func (r *Registry) Execute(ctx context.Context, name string, payload CommandPayload) error {
	r.mu.RLock()
	cmd, ok := r.commands[name]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("command %s not found", name)
	}
	return cmd.Execute(ctx, payload)
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

type HelpCommand struct {
	log      *zap.Logger
	registry *Registry
}

func (c *HelpCommand) Name() string { return "help" }

func (c *HelpCommand) Help() string { return "Menampilkan daftar command." }

func (c *HelpCommand) Execute(ctx context.Context, payload CommandPayload) error {
	c.log.Info("command help", zap.Any("args", payload.Args), zap.Strings("commands", c.registry.Names()))
	_ = ctx
	return nil
}

type InfoCommand struct {
	log *zap.Logger
}

func (c *InfoCommand) Name() string { return "info" }

func (c *InfoCommand) Help() string { return "Menampilkan info akun userbot." }

func (c *InfoCommand) Execute(ctx context.Context, payload CommandPayload) error {
	_ = ctx
	c.log.Info("command info", zap.Int64("telegram_id", payload.TelegramID))
	return nil
}

type StatusCommand struct {
	log *zap.Logger
}

func (c *StatusCommand) Name() string { return "status" }

func (c *StatusCommand) Help() string { return "Menampilkan status worker." }

func (c *StatusCommand) Execute(ctx context.Context, payload CommandPayload) error {
	_ = ctx
	c.log.Info("command status", zap.Int64("telegram_id", payload.TelegramID))
	return nil
}

type GoogleCommand struct {
	log *zap.Logger
}

func (c *GoogleCommand) Name() string { return "gg" }

func (c *GoogleCommand) Help() string { return "Pencarian cepat google." }

func (c *GoogleCommand) Execute(ctx context.Context, payload CommandPayload) error {
	_ = ctx
	c.log.Info("command gg", zap.Any("args", payload.Args))
	return nil
}

type ReplyGuardCommand struct {
	log *zap.Logger
}

func (c *ReplyGuardCommand) Name() string { return "rg" }

func (c *ReplyGuardCommand) Help() string { return "Mengatur reply guard." }

func (c *ReplyGuardCommand) Execute(ctx context.Context, payload CommandPayload) error {
	_ = ctx
	c.log.Info("command rg", zap.Int64("telegram_id", payload.TelegramID))
	return nil
}

type BroadcastCommand struct {
	log *zap.Logger
}

func (c *BroadcastCommand) Name() string { return "sg" }

func (c *BroadcastCommand) Help() string { return "Menjadwalkan broadcast." }

func (c *BroadcastCommand) Execute(ctx context.Context, payload CommandPayload) error {
	_ = ctx
	c.log.Info("command sg", zap.Int64("telegram_id", payload.TelegramID), zap.Any("metadata", payload.Metadata))
	return nil
}
