package YnMIrC

import "log"

type EmptyChannelModeHandler struct {}

func (h *EmptyChannelModeHandler) HandleModeChange(channel, setter, modes string, args []string) {
    // Jelenleg nem csinál semmit, csak logol
    log.Printf("[EmptyChannelModeHandler] Mode change on %s by %s: +%s %v", channel, setter, modes, args)
}

func (h *EmptyChannelModeHandler) GetSavedModes(channel string) (string, error) {
    // Még nincs mentett mód, üres stringet ad vissza
    return "", nil
}
