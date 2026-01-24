package owner

import (
	"fmt"
	"strings"
//	"time"
//	"log"
	_ "github.com/mattn/go-sqlite3"
//	"git.ynm.hu/markus/YnM-Go/YnMConfig"
//	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMLang"
//	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)


// handlePluginCommand handles plugin activation/deactivation commands
func (p *YnmAdminPlugin) handlePluginCommand(sender string, parts []string, issuingChannel string) string {
    if len(parts) < 2 {
        return "Usage: !ynm +pluginname [#channel] OR !ynm -pluginname [#channel] OR !ynm plugins [#channel]"
    }

    command := parts[1]
    
    // Handle plugin listing
    if command == "plugins" {
        targetChannel := issuingChannel
        if len(parts) > 2 && strings.HasPrefix(parts[2], "#") {
            targetChannel = parts[2]
        }
        return p.handleListPluginsCommand(sender, targetChannel)
    }
    
    // Handle plugin activation/deactivation
    var operation string
    var pluginName string
    
    if strings.HasPrefix(command, "+") {
        operation = "activate"
        pluginName = command[1:]
    } else if strings.HasPrefix(command, "-") {
        operation = "deactivate"
        pluginName = command[1:]
    } else {
        return "Usage: !ynm +pluginname [#channel] OR !ynm -pluginname [#channel]"
    }
    
    if pluginName == "" {
       return ""// return "Plugin name cannot be empty"
    }
    
    // Determine target channel
    targetChannel := issuingChannel
    if len(parts) > 2 && strings.HasPrefix(parts[2], "#") {
        targetChannel = parts[2]
    }
    
    // Check if user has permission to manage plugins
    if !p.hasPluginManagePermission(sender, targetChannel) {
        return p.GetMessage(sender, YnMLang.MsgAccessDenied)
    }
    
    // Validate plugin exists
    if !p.isValidPlugin(pluginName) {
        return fmt.Sprintf("Unknown plugin: %s. Use '!ynm plugins' to see available plugins.", pluginName)
    }
    
    // Activate or deactivate plugin
    active := operation == "activate"
    if err := p.Db.SetPluginState(pluginName, targetChannel, active); err != nil {
        return fmt.Sprintf("Error %s plugin %s: %v", operation+"ing", pluginName, err)
    }
    
    action := "activated"
    if !active {
        action = "deactivated"
    }
    
    return fmt.Sprintf("Plugin '%s' %s for channel %s", pluginName, action, targetChannel)
}

// handleListPluginsCommand lists plugins and their status for a channel
// This version sends multiple messages by calling the IRC client directly
func (p *YnmAdminPlugin) handleListPluginsCommand(sender string, channel string) string {
    // Get active plugins for the specific channel
    activePlugins, err := p.Db.GetActivePluginsForChannel(channel)
    if err != nil {
        return fmt.Sprintf("Error retrieving active plugins: %v", err)
    }
    
    // Create a map for quick lookup
    activeMap := make(map[string]bool)
    for _, plugin := range activePlugins {
        activeMap[plugin] = true
    }
    
    // List of available plugins (you can expand this based on your actual plugins)
    availablePlugins := []string{"ora", "media", "admin", "nameday", "xp", "weather", "seen", "sms"}
    
    var activeList, inactiveList []string
    for _, plugin := range availablePlugins {
        if activeMap[plugin] {
            activeList = append(activeList, plugin)
        } else {
            inactiveList = append(inactiveList, plugin)
        }
    }
    
    // Send the header message first
    if p.Bot != nil {
        p.Bot.SendMessage(channel, fmt.Sprintf("Plugin status for %s:", channel))
        
        // Send active plugins
        if len(activeList) > 0 {
            p.Bot.SendMessage(channel, fmt.Sprintf("Active: %s", strings.Join(activeList, ", ")))
        }
        
        // Send inactive plugins  
        if len(inactiveList) > 0 {
            p.Bot.SendMessage(channel, fmt.Sprintf("Inactive: %s", strings.Join(inactiveList, ", ")))
        }
        
        // Send usage message
        p.Bot.SendMessage(channel, "Use: !ynm +name to activate, !ynm -name to deactivate")
    }
    
    // Return empty string since we handled the messages directly
    return ""
}

// hasPluginManagePermission checks if user can manage plugins
func (p *YnmAdminPlugin) hasPluginManagePermission(sender string, channel string) bool {
    hostmask := YnMModule.SimplifyHostmask(sender)
    
    // Check if user is admin/owner
    userInfo, err := p.Db.GetUserInfoByHost(hostmask)
    if err == nil && userInfo != nil {
        if userInfo.Role == "admin" || userInfo.Role == "owner" {
            return true
        }
    }
    
    // Check if user has op/halfop in the channel
    channelUser, err := p.Db.GetUserChannelRole(hostmask, channel)
    if err == nil && channelUser != nil {
        if strings.Contains(channelUser.Role, "op") || channelUser.AutoOp {
            return true
        }
    }
    
    return false
}

// isValidPlugin checks if plugin name is valid
func (p *YnmAdminPlugin) isValidPlugin(pluginName string) bool {
    validPlugins := []string{
        "ora", "media", "admin", "nameday", "xp", "weather", 
        "seen", "sms", "status", "vicc", "tamagotchi", "monitor",
        "link", "forum", "hack", "webhook", "joke", "jellyfininfo",
        "git", "imdb", "xes0", "ssh", "nmap", "dns", "chatgpt", 
        "ip", "pinghost", "learn", "youtube", "debug", "ping",
        "webstatus", "huntorrent", "horoscope", "székelyhon",
    }
    
    for _, valid := range validPlugins {
        if pluginName == valid {
            return true
        }
    }
    return false
}

// Update the main HandleMessage method to include plugin commands
// Add this case to the switch statement in HandleMessage method:

/*
Add this case in the main switch statement of HandleMessage method after "del":

        case "+", "-":
            // Handle plugin activation/deactivation when command starts with + or -
            return p.handlePluginCommand(msg.Sender, []string{"ynm", parts[1]}, issuingChannel)
        case "plugins":
            return p.handleListPluginsCommand(msg.Sender, issuingChannel)
*/

// Also add these lines to handle the format: !ynm +ora or !ynm -ora
// In the HandleMessage method, after checking for "ynm" command, add:

/*
// Handle plugin activation/deactivation with + or - prefix
if len(parts) >= 2 && (strings.HasPrefix(parts[1], "+") || strings.HasPrefix(parts[1], "-")) {
    return p.handlePluginCommand(msg.Sender, parts, issuingChannel)
}

// Handle plugins listing command
if len(parts) >= 2 && parts[1] == "plugins" {
    targetChannel := issuingChannel
    if len(parts) > 2 && strings.HasPrefix(parts[2], "#") {
        targetChannel = parts[2]
    }
    return p.handleListPluginsCommand(msg.Sender, targetChannel)
}
*/