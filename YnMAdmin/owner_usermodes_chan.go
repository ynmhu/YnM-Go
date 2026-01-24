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

package owner

import (
	"fmt"
	"strings"
//	"time"
//	"log"
	_ "github.com/mattn/go-sqlite3"
//	"git.ynm.hu/markus/YnM-Go/YnMConfig"
//	"git.ynm.hu/markus/YnM-Go/YnMIrC"
//	"git.ynm.hu/markus/YnM-Go/YnMLang"
//	"git.ynm.hu/markus/YnM-Go/YnMDb"
//	"git.ynm.hu/markus/YnM-Go/YnMModule"
)


func (p *YnmAdminPlugin) SetChannelMode(issuerFullHostmask, issuingChannel, modeCmd string, targetNicks []string) string {

    issuerNick := strings.Split(issuerFullHostmask, "!")[0]
    
    // ✅ SESSION KEZELÉS - ha be van jelentkezve, akkor azzal a userrel dolgozunk
    effectiveUser, effectiveHost := p.GetEffectiveUser(issuerFullHostmask)
    fmt.Printf("[DEBUG Session] Original: %s -> Effective: %s (%s)\n", issuerNick, effectiveUser, effectiveHost)
    
    channelLower := strings.ToLower(issuingChannel)
    
    // ✅ CSAK A CSATORNA-SPECIFIKUS role (ebben a csatornában milyen jogom van)
    issuerRole, err := p.Db.GetUserRoleInChannel(effectiveUser, effectiveHost, channelLower)
    fmt.Printf("[DEBUG] Channel role for %s in %s: %s, error: %v\n", effectiveUser, channelLower, issuerRole, err)
    
    // ✅ DEBUG VÉGLEGES
    fmt.Printf("[DEBUG SetChannelMode] FINAL - effectiveUser: %s, channelRole: %s\n", 
        effectiveUser, issuerRole)
    
    isUndernet := strings.Contains(issuerFullHostmask, "undernet.org")
    
    if isUndernet && modeCmd == "h" {
        return ""
    }
    
    canSet := false
    issuerRoleLower := strings.ToLower(issuerRole)
    issuerLevel := RoleHierarchy[issuerRoleLower]
    
    switch modeCmd {
    case "o":
        canSet = issuerLevel >= RoleHierarchy["mod"]  // admin vagy owner vagy mod
    case "h":
        canSet = issuerLevel >= RoleHierarchy["mod"]    // mod, admin vagy owner
    case "v":
        canSet = issuerLevel >= RoleHierarchy["vip"]    // vip, mod, admin vagy owner
    default:
        return ""
    }
    
	fmt.Printf("[DEBUG] Channel-specific permission: %t (role: %s, level: %d)\n", canSet, issuerRole, issuerLevel)

	if !canSet {
		return "🔐 Nincs jogosultságod ehhez a parancshoz!"
	}

	if len(targetNicks) < 1 {
		targetNicks = []string{issuerNick}
	}

	var responses []string

	for _, target := range targetNicks {
		// Operátor (+o) és egyéb módok kezelése
		currentModes := p.Bot.GetUserModes(issuingChannel, target)
		isRemoving := strings.Contains(currentModes, modeCmd)
		if strings.EqualFold(target, p.Bot.GetNick()) && isRemoving {
			continue
		}
		
		if modeCmd == "v" {
			modeChar := "+"
			if isRemoving {
				modeChar = "-"
			}
			p.Bot.SendRaw(fmt.Sprintf("MODE %s %s %s", issuingChannel, modeChar+modeCmd, target))
			continue
		}
		
		// Ha saját magának adja/veszi el
		if target == effectiveUser {
			modeChar := "+"
			if isRemoving {
				modeChar = "-"
			}
			p.Bot.SendRaw(fmt.Sprintf("MODE %s %s %s", issuingChannel, modeChar+modeCmd, target))
			continue
		}
		
		// Target információ lekérése
		targetHostmask, _ := p.Db.GetUserHostmaskInChannel(target, channelLower)
		var targetRole string
		var targetLevel int
		
		if targetHostmask != "" {
			targetRole, _ = p.Db.GetUserRoleInChannel(target, targetHostmask, channelLower)
			targetRoleLower := strings.ToLower(targetRole)
			targetLevel = RoleHierarchy[targetRoleLower]
		} else {
			// Ha nincs az adatbázisban, akkor "none" rangú (nincs joga)
			targetRole = "none"
			targetLevel = RoleHierarchy["none"]
		}
		
		fmt.Printf("[DEBUG] Hierarchy check - Issuer: %s (%d), Target: %s (%d), Removing: %t\n", 
			issuerRole, issuerLevel, targetRole, targetLevel, isRemoving)

		// ⭐ OP JOG KEZELÉSE ⭐
		if modeCmd == "o" {
			// OP jog adása (+o) - ADÁS
			if !isRemoving {
				if strings.EqualFold(target, p.Bot.GetNick()) {
					continue
				}
				if issuerRole == "admin" || issuerRole == "owner" {
				}else if issuerRole == "mod" {
						if targetRole == "admin" || targetRole == "owner" {
				} else {
						responses = append(responses, 
							fmt.Sprintf("❌ Nincs jogosultságod OP jogot adni ennek a felhasználónak: %s (%s)", 
							target, targetRole))
						continue
					}
				} else {
					responses = append(responses, 
						fmt.Sprintf("❌ Nincs jogosultságod OP jogot adni: %s", target))
					continue
				}
				} else { 
					if issuerRole == "admin" {
						if targetRole == "owner" {
							responses = append(responses, 
								fmt.Sprintf("❌ Owner-től nem veheted el az OP jogot: %s", target))
							continue
						}                   
						if targetRole == "admin" {
							responses = append(responses, 
								fmt.Sprintf("❌ Másik Admin-tól nem veheted el az OP jogot: %s", target))
							continue
						}
					} else if issuerRole == "owner" {
						// Owner deopolhat bárkit (adminokat is)
						// nincs korlátozás
					} else if issuerRole == "mod" {
					if targetRole == "owner" || targetRole == "admin" || targetRole == "mod" {
						responses = append(responses, 
							fmt.Sprintf("❌ Nem veheted el az OP jogot %s rangú felhasználótól: %s", 
							targetRole, target))
						continue
					}
				
				} else {
					responses = append(responses, 
						fmt.Sprintf("❌ Nincs jogosultságod OP jogot elvenni: %s", target))
					continue
				}
			}
		}
		modeChar := "+"
		if isRemoving {
			modeChar = "-"
		}		
		p.Bot.SendRaw(fmt.Sprintf("MODE %s %s %s", issuingChannel, modeChar+modeCmd, target))
	}

	if len(responses) > 0 {
		return strings.Join(responses, " | ")
	}
	return ""
}