# YnM-Go Permission System ⚡

## Role Hierarchy 🏰
**OWNER (5) > ADMIN (4) > MOD (3) > VIP (2) > USER (1)**

---

## 1. GLOBAL PERMISSIONS 🌐
*Only OWNER can give global permissions!*

**Commands:**
- 🎯 `!ynm add vip Username` → Global VIP (all channels)  
- 🛡️ `!ynm add mod Username` → Global Mod (all channels)  
- 🔑 `!ynm add admin Username` → Global Admin (all channels)  

**Access:**
- 🌍 All YNM commands in ALL channels  
- 🤖 Can use bot anywhere they join  

**Example:**  
🎯 `!ynm add vip TestUser`  
_Result: TestUser can use !v, !h, !o in #chan1, #chan2, #chan3..._

---

## 2. LOCAL PERMISSIONS 📌
*Permissions are channel-specific*

**Who can give local permissions?**
- 👑 **OWNER:** can give VIP, MOD, ADMIN in any channel  
- 🛡️ **ADMIN:** can give VIP, MOD (not other ADMINS or OWNER)  
- ⚔️ **MOD:** can give VIP only (not MODS, ADMINS or OWNER)  
- ✋ **VIP:** cannot give any permissions  

**Commands:**
- 🎯 `!ynm add vip #Channel Username` → Local VIP (only in #Channel)  
- 🛡️ `!ynm add mod #Channel Username` → Local Mod (only in #Channel)  
- 🔑 `!ynm add admin #Channel Username` → Local Admin (only in #Channel)  

**Access:**
- 📌 Only in specified channel  
- 🚫 No access in other channels  

**Example:**  
🎯 `!ynm add vip #Teszt User123`  
_Result: User123 can use !v only in #Teszt channel. In #OtherChannel: no bot access!_

---

## 3. COMMAND ACCESS BY ROLE 🎮

**Mode Commands:**
- 🎤 `!v` (voice) → VIP, MOD, ADMIN, OWNER  
- ✋ `!h` (halfop) → MOD, ADMIN, OWNER  
- 👑 `!o` (op) → ADMIN, OWNER  
- 📝 `!t` (topic) → MOD, ADMIN, OWNER  

**Bot Access Rule:**
- 🔑 VIP or higher role required to use bot commands  
- 📌 Role must be in THAT channel (local or global)

---

## 4. PRACTICAL EXAMPLES 📝

**Example 1: Local VIP**  
🎯 `!ynm add vip #GameRoom Player1`  
- ✅ Player1 can use !v in #GameRoom  
- ❌ Cannot use bot in #MusicRoom  
- ❌ Cannot use !h or !o anywhere  

**Example 2: Global Mod**  
🛡️ `!ynm add mod Moderator99`  
- ✅ Can use !v, !h in ALL channels  
- ✅ Can give local VIP in channels where they have MOD role  
- ❌ Cannot use !o (needs ADMIN)  

**Example 3: Local Admin**  
🔑 `!ynm add admin #Support HelpDesk`  
- ✅ Can use !v, !h, !o in #Support  
- ✅ Can give local VIP/MOD in #Support  
- ❌ Cannot use bot in #OtherChannel  

**Example 4: Global VIP**  
🎯 `!ynm add vip VIPUser`  
- ✅ Can use !v in ALL channels  
- ❌ Cannot give any permissions  
- ❌ Cannot use !h or !o

---

## 5. IMPORTANT RULES ⚠️
- 🌍 Global roles override local restrictions  
- 👑 Higher roles include lower role commands (Admin can use !v, !h, !o)  
- 📌 Permissions are checked per-channel  
- ❌ No role = no bot access in that channel  
- 👑 Owner can do everything everywhere

---

## 6. TROUBLESHOOTING 🛠️

**User can't use bot in channel?**
- ✅ Check if they have VIP+ role in that channel  
- ✅ Check if they have global VIP+ role  
- ✅ Check if bot is active in that channel  

**User can't give permissions?**
- ✅ Check if they have required role (MOD for VIP, ADMIN for MOD, etc.)  
- ✅ Check if they're trying to give equal/higher role  
- ✅ Check if they're in the right channel  

**Bot not responding?**
- ✅ Check if user has at least VIP role in that channel  
- ✅ Check if command prefix is correct (! or botnick)  
- ✅ Check bot logs for errors


#LIVE
[20:29:03] (Mod): !ynm add vip #YnM Vip
[20:29:03] (YnM-Beta): 📍 Lokális VIP jog hozzáadása folyamatban 1 felhasználóhoz (#ynm)...
[20:29:04] *** YnM-Beta sets mode: +v Vip
[20:29:04] (YnM-Beta): ✅ Lokális VIP jog hozzáadva: Vip -> #ynm
[20:29:07] (Mod): !ynm del vip #YnM Vip
[20:29:07] (YnM-Beta): 📍 Lokális VIP jog törlése folyamatban 1 felhasználótól (#ynm)...
[20:29:07] (YnM-Beta): ✅ VIP jog elvéve: Vip @ #ynm (törölte: Mod)
[20:29:08] *** YnM-Beta sets mode: -v Vip
[20:29:09] (Mod): !ynm add vip #YnM Vip
[20:29:09] (YnM-Beta): 📍 Lokális VIP jog hozzáadása folyamatban 1 felhasználóhoz (#ynm)...
[20:29:09] (YnM-Beta): ✅ VIP jog visszaadva: Vip @ #ynm (felelős: Mod)
[20:29:10] *** YnM-Beta sets mode: +v Vip
[20:29:10] (YnM-Beta): ✅ VIP jog visszaadva: Vip @ #ynm (hozzáadta: Mod, most módosította: Mod)
[20:29:12] (Admin): !ynm del vip #YnM Vip
[20:29:12] (YnM-Beta): 📍 Lokális VIP jog törlése folyamatban 1 felhasználótól (#ynm)...
[20:29:12] (YnM-Beta): ✅ VIP jog elvéve: Vip @ #ynm (módosította: Admin, eredeti hozzáadó: Mod)
[20:29:13] *** YnM-Beta sets mode: -v Vip
[20:29:18] (Admin): !ynm add vip #YnM Vip
[20:29:18] (YnM-Beta): 📍 Lokális VIP jog hozzáadása folyamatban 1 felhasználóhoz (#ynm)...
[20:29:19] (YnM-Beta): ✅ VIP jog visszaadva és átvették a jogosultságokat: Vip @ #ynm (új felelős: Admin, korábbi felelős: Mod)
[20:29:19] *** YnM-Beta sets mode: +v Vip
[20:29:20] (YnM-Beta): ✅ VIP jog visszaadva: Vip @ #ynm (hozzáadta: Mod, most módosította: Admin)
[20:29:25] (Mod): !ynm del vip #YnM Vip
[20:29:25] (YnM-Beta): 📍 Lokális VIP jog törlése folyamatban 1 felhasználótól (#ynm)...
[20:29:26] (YnM-Beta): ❌ Nem veheted el a VIP jogot: Vip (hozzáadta: Admin, role: admin, te: mod)
[20:29:27] (Mod): !ynm add vip #YnM Vip
[20:29:27] (YnM-Beta): 📍 Lokális VIP jog hozzáadása folyamatban 1 felhasználóhoz (#ynm)...
[20:29:27] (YnM-Beta): ❌ Nem adhatod vissza a VIP jogot: Vip (hozzáadta: Admin, utoljára módosította: Admin)
