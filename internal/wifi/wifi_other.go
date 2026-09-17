//go:build !darwin

package wifi

import (
	"context"
	"errors"
	"time"
)

// Keeper is a no-op off macOS.
type Keeper struct{}

func New(ssid string, dryRun bool, bridge *AppBridge) *Keeper { return &Keeper{} }
func (k *Keeper) SetOnlineChecker(func() bool)                {}
func (k *Keeper) Device() string                              { return "" }
func (k *Keeper) SSID() string                                { return "" }
func (k *Keeper) LastAction() (string, string, time.Time)     { return "", "", time.Time{} }
func (k *Keeper) Tick(ctx context.Context, online bool, since time.Time) {}
func Password(ssid string) (string, error)                      { return "", errors.New("unsupported") }
func SetPassword(ssid, pw string) error                         { return errors.New("unsupported") }
