// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Fail2Ban IRC plugin
// ==================================================
package ynm

import (
	"bufio"
	"fmt"
//	"log"
	"os"
	"regexp"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

type Fail2BanPlugin struct {
	client  *YnMIrC.Client
	logFile string
	channels []string
	stop    chan struct{}
}

func NewFail2BanPlugin(client *YnMIrC.Client, logFile string, channels []string) *Fail2BanPlugin {
	return &Fail2BanPlugin{
		client:  client,
		logFile: logFile,
		channels: channels, // Javítás: a paramétert használjuk
		stop:    make(chan struct{}),
	}
}

func (p *Fail2BanPlugin) Load() error {
	// Check if log file exists and is readable
	if _, err := os.Stat(p.logFile); os.IsNotExist(err) {
		return fmt.Errorf("log fájl nem létezik: %s", p.logFile)
	}
	
	// Test if we can read the file
	file, err := os.Open(p.logFile)
	if err != nil {
		return fmt.Errorf("log fájl nem olvasható: %v", err)
	}
	file.Close()
	
	go p.watchLog()
	//log.Printf("Fail2Ban plugin betöltve, csatornák: %v", p.channels)
	return nil
}

func (p *Fail2BanPlugin) Unload() error {
	close(p.stop)
	return nil
}

func (p *Fail2BanPlugin) watchLog() {
	fi, err := os.Stat(p.logFile)
	if err != nil {
		return
	}
	lastSize := fi.Size()
	
	// Compile regex once for better performance
	banRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2},\d{3})\s+fail2ban\.actions\s+\[\d+\]:\s+(NOTICE|WARNING)\s+\[(\w+)\]\s+(Ban|Unban|already banned)\s+([0-9\.]+)`)

	for {
		select {
		case <-p.stop:
			return
		default:
			fi, err := os.Stat(p.logFile)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			size := fi.Size()
			if size < lastSize {
				// Log rotated, reset position
				lastSize = 0
			}

			if size > lastSize {
				file, err := os.Open(p.logFile)
				if err != nil {
					time.Sleep(2 * time.Second)
					continue
				}

				_, err = file.Seek(lastSize, 0)
				if err != nil {
					file.Close()
					time.Sleep(2 * time.Second)
					continue
				}

				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					
					matches := banRegex.FindStringSubmatch(line)
					if len(matches) >= 6 {
						timestamp := matches[1]
						level := matches[2]
						jail := matches[3]
						action := matches[4]
						ip := matches[5]

						msg := fmt.Sprintf("[Fail2Ban] %s %s: %s (jail=%s, ip=%s)", 
							timestamp, level, action, jail, ip)
						
						// Javítás: minden csatornába küldjük az üzenetet
						for _, channel := range p.channels {
							p.client.SendMessage(channel, msg)
						}
					}
				}

				file.Close()
				lastSize = size
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// Plugin interface methods
func (p *Fail2BanPlugin) Name() string {
	return "Fail2Ban"
}

func (p *Fail2BanPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *Fail2BanPlugin) OnTick() []YnMIrC.Message {
	return nil
}