// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ==================================================

package discord

import (
	"fmt"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"github.com/bwmarrin/discordgo"
)

type DiscordInfoPlugin struct {
	bot     *YnMIrC.Client
	session *discordgo.Session
}

func NewDiscordInfoPlugin(bot *YnMIrC.Client, session *discordgo.Session) *DiscordInfoPlugin {
	return &DiscordInfoPlugin{
		bot:     bot,
		session: session,
	}
}

func (p *DiscordInfoPlugin) Name() string {
	return "DiscordInfoPlugin"
}

func (p *DiscordInfoPlugin) Commands() []string {
	return []string{"!who", "!discordinfo"}
}

func (p *DiscordInfoPlugin) Help() string {
	return "!who - Megmutatja, mit lát a Discord bot a környezetedről"
}

func (p *DiscordInfoPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	
	if !strings.HasPrefix(strings.ToLower(text), "!who") {
		return ""
	}

	// Csak Discord csatornákra válaszoljunk (IRC csatornák #-el kezdődnek)
	if strings.HasPrefix(msg.Channel, "#") {
		return ""
	}

	return p.buildInfoResponse(msg)
}

func (p *DiscordInfoPlugin) buildInfoResponse(msg YnMIrC.Message) string {
	var sb strings.Builder
	
	sb.WriteString("**🤖 Discord Bot látóköröm:**\n\n")
	
	// Session ellenőrzés
	if p.session == nil || p.session.State == nil {
		sb.WriteString("**❌ Discord session nem elérhető**\n\n")
		p.appendUserBasicInfo(&sb, msg)
		sb.WriteString(fmt.Sprintf("\n*⏰ Lekérve: %s*", time.Now().Format("2006-01-02 15:04:05")))
		return sb.String()
	}
	
	// Bot státusz információk
	p.appendBotInfo(&sb)
	
	// Channel és Guild információk
	channel, guild := p.getChannelAndGuild(msg.Channel)
	if channel != nil {
		p.appendChannelInfo(&sb, channel)
		
		if guild != nil {
			p.appendGuildInfo(&sb, guild)
			p.appendMemberInfo(&sb, guild, msg.Nick)
		}
	} else {
		sb.WriteString("**📢 Csatorna:** `Nem található`\n")
	}
	
	// User alapinformációk
	p.appendUserBasicInfo(&sb, msg)
	
	// Timestamp
	sb.WriteString(fmt.Sprintf("\n*⏰ Lekérve: %s*", time.Now().Format("2006-01-02 15:04:05")))
	
	return sb.String()
}

func (p *DiscordInfoPlugin) appendBotInfo(sb *strings.Builder) {
	if p.session.State == nil || p.session.State.User == nil {
		return
	}
	
	sb.WriteString("**🤖 Bot státusz:**\n")
	sb.WriteString(fmt.Sprintf("• Név: `%s#%s`\n", 
		p.session.State.User.Username, 
		p.session.State.User.Discriminator))
	sb.WriteString(fmt.Sprintf("• ID: `%s`\n", p.session.State.User.ID))
	
	guildCount := len(p.session.State.Guilds)
	sb.WriteString(fmt.Sprintf("• Aktív szerverek: `%d`\n\n", guildCount))
}

func (p *DiscordInfoPlugin) getChannelAndGuild(channelID string) (*discordgo.Channel, *discordgo.Guild) {
	channel, err := p.session.State.Channel(channelID)
	if err != nil {
		return nil, nil
	}
	
	if channel.GuildID == "" {
		return channel, nil
	}
	
	guild, err := p.session.State.Guild(channel.GuildID)
	if err != nil {
		return channel, nil
	}
	
	return channel, guild
}

func (p *DiscordInfoPlugin) appendChannelInfo(sb *strings.Builder, channel *discordgo.Channel) {
	sb.WriteString("**📢 Csatorna információk:**\n")
	sb.WriteString(fmt.Sprintf("• Név: `%s`\n", channel.Name))
	sb.WriteString(fmt.Sprintf("• ID: `%s`\n", channel.ID))
	sb.WriteString(fmt.Sprintf("• Típus: `%s`\n", p.getChannelTypeName(channel.Type)))
	
	if channel.Topic != "" {
		topic := channel.Topic
		if len(topic) > 100 {
			topic = topic[:97] + "..."
		}
		sb.WriteString(fmt.Sprintf("• Téma: `%s`\n", topic))
	}
	sb.WriteString("\n")
}

func (p *DiscordInfoPlugin) getChannelTypeName(channelType discordgo.ChannelType) string {
	switch channelType {
	case discordgo.ChannelTypeGuildText:
		return "Szöveges csatorna"
	case discordgo.ChannelTypeDM:
		return "Privát üzenet"
	case discordgo.ChannelTypeGuildVoice:
		return "Hang csatorna"
	case discordgo.ChannelTypeGroupDM:
		return "Csoportos privát"
	case discordgo.ChannelTypeGuildCategory:
		return "Kategória"
	case discordgo.ChannelTypeGuildNews:
		return "Hírek"
	case discordgo.ChannelTypeGuildStore:
		return "Bolt"
	case discordgo.ChannelTypeGuildNewsThread:
		return "Hír szál"
	case discordgo.ChannelTypeGuildPublicThread:
		return "Nyilvános szál"
	case discordgo.ChannelTypeGuildPrivateThread:
		return "Privát szál"
	case discordgo.ChannelTypeGuildStageVoice:
		return "Színpad"
	default:
		return fmt.Sprintf("Ismeretlen (%d)", channelType)
	}
}

func (p *DiscordInfoPlugin) appendGuildInfo(sb *strings.Builder, guild *discordgo.Guild) {
	sb.WriteString("**🏰 Szerver információk:**\n")
	sb.WriteString(fmt.Sprintf("• Név: `%s`\n", guild.Name))
	sb.WriteString(fmt.Sprintf("• ID: `%s`\n", guild.ID))
	sb.WriteString(fmt.Sprintf("• Tagok: `%d`\n", guild.MemberCount))
	sb.WriteString(fmt.Sprintf("• Rollok: `%d`\n", len(guild.Roles)))
	sb.WriteString(fmt.Sprintf("• Csatornák: `%d`\n", len(guild.Channels)))
	
	if guild.OwnerID != "" {
		sb.WriteString(fmt.Sprintf("• Tulajdonos ID: `%s`\n", guild.OwnerID))
	}
	sb.WriteString("\n")
}

func (p *DiscordInfoPlugin) appendMemberInfo(sb *strings.Builder, guild *discordgo.Guild, userID string) {
	member, err := p.session.State.Member(guild.ID, userID)
	if err != nil {
		sb.WriteString("**👤 Felhasználó:** `Információ nem elérhető`\n\n")
		return
	}
	
	sb.WriteString("**👤 Felhasználó részletek:**\n")
	
	// Felhasználónév
	if member.User != nil {
		sb.WriteString(fmt.Sprintf("• Felhasználónév: `%s#%s`\n", 
			member.User.Username, 
			member.User.Discriminator))
	}
	
	// Becenév
	if member.Nick != "" {
		sb.WriteString(fmt.Sprintf("• Becenév: `%s`\n", member.Nick))
	}
	
	// Csatlakozás ideje
	if !member.JoinedAt.IsZero() {
		joinDuration := time.Since(member.JoinedAt)
		days := int(joinDuration.Hours() / 24)
		sb.WriteString(fmt.Sprintf("• Csatlakozás: `%s` (%d napja)\n", 
			member.JoinedAt.Format("2006-01-02 15:04"), 
			days))
	}
	
	// Boosting státusz
	if !member.PremiumSince.IsZero() {
		sb.WriteString(fmt.Sprintf("• 🚀 Booster: `%s` óta\n", 
			member.PremiumSince.Format("2006-01-02")))
	}
	
	// Rollok
	p.appendMemberRoles(sb, guild, member)
	
	sb.WriteString("\n")
}

func (p *DiscordInfoPlugin) appendMemberRoles(sb *strings.Builder, guild *discordgo.Guild, member *discordgo.Member) {
	if len(member.Roles) == 0 {
		return
	}
	
	sb.WriteString("• Rollok: ")
	
	roleNames := make([]string, 0, len(member.Roles))
	for _, roleID := range member.Roles {
		role, err := p.session.State.Role(guild.ID, roleID)
		if err == nil {
			roleNames = append(roleNames, role.Name)
		}
	}
	
	maxRoles := 5
	if len(roleNames) > maxRoles {
		sb.WriteString(fmt.Sprintf("`%s`", strings.Join(roleNames[:maxRoles], "`, `")))
		sb.WriteString(fmt.Sprintf(" és még `%d` másik", len(roleNames)-maxRoles))
	} else {
		sb.WriteString(fmt.Sprintf("`%s`", strings.Join(roleNames, "`, `")))
	}
	sb.WriteString("\n")
}

func (p *DiscordInfoPlugin) appendUserBasicInfo(sb *strings.Builder, msg YnMIrC.Message) {
	sb.WriteString("**📝 Üzenet információk:**\n")
	sb.WriteString(fmt.Sprintf("• Felhasználó ID: `%s`\n", msg.Nick))
	sb.WriteString(fmt.Sprintf("• Csatorna ID: `%s`\n", msg.Channel))
	
	msgText := msg.Text
	if len(msgText) > 50 {
		msgText = msgText[:47] + "..."
	}
	sb.WriteString(fmt.Sprintf("• Üzenet: `%s`\n", msgText))
}

func (p *DiscordInfoPlugin) OnTick() []YnMIrC.Message {
	return nil
}