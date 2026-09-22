package modes

import (
	"strconv"

	"github.com/patriceckhart/zot/packages/tui"
)

// Optional to preserve compatibility with embedders implementing SettingsStore.
type chatTimestampSettingsStore interface {
	SetChatTimestamps(bool) error
	SetChatTimestampInterval(int) error
}

type chatTimestampDateSettingsStore interface {
	SetChatTimestampDate(string) error
}

func (i *Interactive) chatTimestampDateSetting() settingsItem {
	choice := 0
	if tui.ChatTimestampDateMode(i.cfg.ChatTimestampDate) == "every_message" {
		choice = 1
	}
	return settingsItem{
		key:    "chat_timestamp_date",
		label:  "date display",
		desc:   "choose when to include the date beside the time",
		choice: choice,
		options: []settingsOption{
			{value: "day_start", label: "Start of each day", desc: "show the date on the first timestamp and when the day changes"},
			{value: "every_message", label: "Every timestamp", desc: "include the date on every displayed timestamp"},
		},
	}
}

func (i *Interactive) applyChatTimestampDate(value string) {
	mode := tui.ChatTimestampDateMode(value)
	if store, ok := i.cfg.SettingsStore.(chatTimestampDateSettingsStore); ok {
		if err := store.SetChatTimestampDate(mode); err != nil {
			i.chatTimestampSettingError(err)
			return
		}
	}
	i.mu.Lock()
	i.cfg.ChatTimestampDate = mode
	i.view.ChatTimestampDate = mode
	i.statusOK = "timestamp date display updated"
	i.statusErr = ""
	i.mu.Unlock()
	i.repaintChatTimestamps()
}

func (i *Interactive) chatTimestampIntervalSetting() settingsItem {
	minutes := tui.ChatTimestampInterval(i.cfg.ChatTimestampIntervalMinutes)
	item := settingsItem{
		key:   "chat_timestamp_interval_minutes",
		label: "interval",
		desc:  "minimum minutes since the last timestamp, zero shows every completed message",
	}
	values := []int{0, 1, 5, 10, 15, 30, 60}
	found := false
	for _, value := range values {
		found = found || value == minutes
	}
	if !found {
		values = append(values, minutes)
	}
	for idx, value := range values {
		label := strconv.Itoa(value) + " min"
		if value == 0 {
			label = "every message"
		}
		item.options = append(item.options, settingsOption{value: strconv.Itoa(value), label: label})
		if value == minutes {
			item.choice = idx
		}
	}
	return item
}

func (i *Interactive) applyChatTimestamps(enabled bool) {
	if store, ok := i.cfg.SettingsStore.(chatTimestampSettingsStore); ok {
		if err := store.SetChatTimestamps(enabled); err != nil {
			i.chatTimestampSettingError(err)
			return
		}
	}
	i.mu.Lock()
	i.cfg.ChatTimestamps = enabled
	i.view.ChatTimestamps = enabled
	i.statusOK = "chat timestamps " + onOff(enabled)
	i.statusErr = ""
	i.mu.Unlock()
	i.repaintChatTimestamps()
}

func (i *Interactive) applyChatTimestampInterval(value string) {
	minutes, err := strconv.Atoi(value)
	if err != nil || minutes < 0 {
		return
	}
	if store, ok := i.cfg.SettingsStore.(chatTimestampSettingsStore); ok {
		if err := store.SetChatTimestampInterval(minutes); err != nil {
			i.chatTimestampSettingError(err)
			return
		}
	}
	i.mu.Lock()
	i.cfg.ChatTimestampIntervalMinutes = &minutes
	i.view.ChatTimestampIntervalMinutes = &minutes
	i.statusOK = "chat timestamp interval " + value + " min"
	i.statusErr = ""
	i.mu.Unlock()
	i.repaintChatTimestamps()
}

func (i *Interactive) chatTimestampSettingError(err error) {
	i.mu.Lock()
	i.statusErr = "settings: " + err.Error()
	i.statusOK = ""
	i.mu.Unlock()
	i.invalidate()
}

func (i *Interactive) repaintChatTimestamps() {
	if i.rend != nil {
		i.rend.Clear()
	}
	i.invalidate()
}
