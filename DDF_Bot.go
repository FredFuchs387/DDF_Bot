package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/textproto"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Connection type allows for easier joining the twitch IRC server as well as maintaining the connection
type Connection struct {
	conn net.Conn
}

type Chatter struct {
	name   string
	time   time.Time
	banDur int
	banCt  int
}

var userListMutex = &sync.RWMutex{}
var userList = map[string]*Chatter{}

var userMatch = regexp.MustCompile(`\W:([^!]+)!`)
var flagMatch = regexp.MustCompile(`@.+?\s:`)
var msgMatch = regexp.MustCompile("PRIVMSG #vansamaofficial :(.*)$")

var charMatch = regexp.MustCompile("[Ѐ-ӿ]+")
var botCharMatch = regexp.MustCompile("[Ꭰ-Ᏼ]+")
var rBotStringMatch = regexp.MustCompile("зачистка соледара прошла успешно, гойда!")
var lenMatch = regexp.MustCompile("^.{400,}$")
var urlMatch = regexp.MustCompile(`http(s?)://`)
var onlineMatch = regexp.MustCompile(`(?i)@your___m0m YOURM0M`)
var bitMatch = regexp.MustCompile(`;bits=[0-9]+;.+?\s`)
var subMatch = regexp.MustCompile(`;msg-param-cumulative-months=[0-9]+;.+?:`)
var numMatch = regexp.MustCompile(`[0-9]+`)
var modMatch = regexp.MustCompile(`;badges=moderator.+?\s`)
var vipMatch = regexp.MustCompile(`;badges=vip.+?\s`)
var timezone = regexp.MustCompile(`[0-9]\s?(?:[ap]m)? *est`)
var merchLastTime = time.Now().Add(time.Second * -20)
var socialLastTime = time.Now().Add(time.Second * -20)

// Moderator Commands
var nukeOnMatch = regexp.MustCompile(`(?i)(^)!NukeOn($)`)
var nukeOffMatch = regexp.MustCompile(`(?i)(^)!NukeOff($)`)
var mediashare = regexp.MustCompile(`(?i)(^)!mediashareon($)`)
var mediashareOff = regexp.MustCompile(`(?i)(^)!mediashareoff($)`)
var russianOn = regexp.MustCompile(`(?i)(^)!russianon($)`)
var russianOff = regexp.MustCompile(`(?i)(^)!russianoff($)`)

// Chat Commands
var merch = regexp.MustCompile(`(?i)(^)!merch($)`)
var social = regexp.MustCompile(`(?i)(^)!social($)`)
var shoutout = regexp.MustCompile(`(?i)(^)!shoutout($)`)
var shoutoutru = regexp.MustCompile(`(?i)(^)!shoutoutru($)`)

// Shoutout Filters
var so1 = regexp.MustCompile(`(?i)say(\W)`)
var so2 = regexp.MustCompile(`(?i)hello (to)?`)
var so3 = regexp.MustCompile(`(?i)hi (to)?`)
var so4 = regexp.MustCompile(`(?i)can you`)
var so5 = regexp.MustCompile(`(?i)please`)
var so6 = regexp.MustCompile(`(?i)congratulate`)
var so7 = regexp.MustCompile(`(?i)wish`)
var so8 = regexp.MustCompile(`(?i)birthday`)

// Current Event Filters
var ce1 = regexp.MustCompile(`(?i)(\W)ukraine`)
var ce2 = regexp.MustCompile(`(?i)(\W)russia`)
var ce3 = regexp.MustCompile(`(?i)(\W)war`)
var ce4 = regexp.MustCompile(`(?i)(\W)ww3`)

// Merchandise Links
var mywheats = `https://www.mywheats.com/vansamaofficial`
var slmerch = `https://streamlabs.com/vansamaofficial/merch`
var taobao = `https://shop170176806.world.taobao.com/index.htm?spm=2013.1.w5002-23239319357.2.5faa46bdnfP0oX`

// Social Media Links
var bilibili = `https://space.bilibili.com/477631979`
var instagram = `https://www.instagram.com/vansamaofficial/`
var twitter = `https://twitter.com/vansamaofficial`
var youtube = `https://www.youtube.com/c/vansamaofficial`

// Default state of Nuke is OFF
var nukeState = false

// Default state of Media Share Notifications is OFF
var mediaState = false

// Default state of Russian language allowed is OFF
var russianState = false

// tosSlice contains strings which violate/risk violating Twitch TOS
var tosSlice = []string{
	wordMatcher(`fag`),
	`(?i)(\W|^)(n\W*|И\W*)i\W*(g\W*)+(e\W*|y\W*)?r`,
	`(?i)(\W|^)(n\W*|И\W*)(i\W*|y\W*)(g\W*)+(\W|$|a)`,
	`(?i)p\W*(i\W*|e\W*)d\W*(o\W*|a\W*)*r\W*`,
	wordMatcher(`peedor`),
	wordMatcher(`peedour`),
	wordMatcher(`pidrila`),
	`(?i)п\PL*(и\PL*|й\PL*)д\PL*(о\PL*|а\PL*)р`,
	`(?i)п\PL*е\PL*д\PL*и\PL*к`,
	`(?i)н\PL*(и\PL*|й\PL*|е\PL*)г+\PL*(е\PL*|а\PL*)*р`,
	wordMatcher(`retard`),
	wordMatcher(`tranny`),
}
var tosMatch = regexp.MustCompile("(?:(?:" + strings.Join(tosSlice, ")|(?:") + "))")

// otherLangSlice contains non English strings which are to be filtered
var otherLangSlice = []string{
	wordMatcherEndL(`bez`),
	wordMatcherEndL(`cherez`),
	wordMatcherEndL(`cho`),
	wordMatcherEndL(`chto`),
	wordMatcherEndL(`dela`),
	wordMatcherEndL(`ebani`),
	wordMatcherEndL(`ebat`),
	wordMatcherEndL(`ectb`),
	wordMatcherEndL(`est`),
	wordMatcherEndL(`est'`),
	wordMatcherEndL(`estb`),
	wordMatcherEndL(`eto`),
	wordMatcherEndL(`iz`),
	wordMatcherEndL(`kak`),
	wordMatcher(`kaifovo`),
	wordMatcherEndL(`kto`),
	wordMatcher(`kogda`),
	wordMatcher(`meste`),
	wordMatcherEndL(`nad`),
	wordMatcher(`pizdec`),
	wordMatcher(`pochemu`),
	wordMatcher(`po4emy`),
	wordMatcher(`poimal`),
	wordMatcher(`posle`),
	wordMatcher(`pered`),
	wordMatcher(`russkie`),
	wordMatcher(`ruskie`),
	wordMatcher(`ruskim`),
	wordMatcher(`slava`),
	wordMatcherEndL(`tut`),
	wordMatcherEndL(`tyt`),
	wordMatcher(`vpered`),
	wordMatcher(`vperde`),
	wordMatcher(`vsem`),
	wordMatcher(`wsem`),
	wordMatcherEndL(`vot`),
	wordMatcherEndL(`za`),
	wordMatcherEndL(`zaebis`),
	wordMatcherEndL(`zaebal`),
	`(?i)(z\W*?)d\W*a\W*r\W*o\W*(v\W*|w\W*)a`,
}
var otherLangMatch = regexp.MustCompile("(?:(?:" + strings.Join(otherLangSlice, ")|(?:") + "))")

// spamSlice contains strings which are deemed to be spam
var spamSlice = []string{
	wordMatcher(`stray228`),
	wordMatcherEndL(`wewe`),
	wordMatcherEndL(`veve`),
	wordMatcher(`flexair`),
	wordMatcher(`rat tv`),
	wordMatcherEndL(`aboba`),
}
var spamMatch = regexp.MustCompile("(?:(?:" + strings.Join(spamSlice, ")|(?:") + "))")

// Contains all the possible !8ball responses
var ballSlice = []string{
	`As I see it, yes.`,
	`Ask again later.`,
	`Better not tell you now.`,
	`Cannot predict now.`,
	`Concentrate and ask again.`,
	`Don't count on it.`,
	`It is certain.`,
	`It is decidedly so.`,
	`Most likely.`,
	`My reply is no.`,
	`My sources say no.`,
	`Outlook not so good.`,
	`Outlook good.`,
	`Reply hazy, try again.`,
	`Signs point to yes.`,
	`Very doubtful.`,
	`Without a doubt.`,
	`Yes.`,
	`Yes - definitely.`,
	`You may rely on it.`,
}

// Matches !8ball at the beginning of a message
var ballMatch = regexp.MustCompile(`(^)!8ball `)

var dungeonMatch = regexp.MustCompile(`(^)!enter(\W|$)`)

// Converts a normal string into a consistent regex pattern
func wordMatcher(word string) string {
	return `(?i)(\W|^)` + strings.Join(strings.Split(word, ""), `+\W*`)
}

// As wordMatcher, except this pattern checks for trailing non-words or endline
func wordMatcherEndL(word string) string {
	return `(?i)(\W|^)` + strings.Join(strings.Split(word, ""), `\W*`) + `(\W|$)`
}

// Checks text extracted from IRC and responds based on the first matched regex
func (c *Connection) chatMod(flags string, usr string, msgText string) {
	if bitMatch.MatchString(flags) {
		bitFlag := bitMatch.FindStringSubmatch(flags)
		bits := numMatch.FindStringSubmatch(bitFlag[0])
		c.sendMsg("/me %v, Thanks for the %v bits FeelsGoodMan Clap", usr, bits[0])
		return
	}

	if subMatch.MatchString(flags) {
		subFlag := subMatch.FindStringSubmatch(flags)
		subCount := numMatch.FindStringSubmatch(subFlag[0])
		c.sendMsg("/me Thanks for the %v months, %v VaN :v:", subCount[0], usr)
		return
	}

	if modMatch.MatchString(flags) {
		if nukeOnMatch.MatchString(msgText) {
			c.sendMsg("YOU GUYS NEED TO RELAX MODS")
			c.sendMsg("/slow 30")
			c.sendMsg("/followers 3d")
			c.sendMsg("/subscribers")
			nukeState = true
			return
		}
		if nukeOffMatch.MatchString(msgText) {
			nukeState = false
			return
		}
		if russianOn.MatchString(msgText) {
			russianState = true
			c.sendMsg("Russian Text ENABLED in Chat @%v", usr)
			return
		}
		if russianOff.MatchString(msgText) {
			russianState = false
			c.sendMsg("Russian Text DISABLED in Chat @%v", usr)
			return
		}
		if onlineMatch.MatchString(msgText) {
			c.sendMsg("@%v YOURM0M", usr)
			return
		}
		if mediashare.MatchString(msgText) {
			mediaState = true
			c.sendMsg("/me Hey chat, Van will play your gachimuchi remixes if you include them in your donation! HandsUp")
			return
		}
		if mediashareOff.MatchString(msgText) {
			mediaState = false
			return
		}
	}

	if nukeState {
		if !modMatch.MatchString(flags) || !vipMatch.MatchString(flags) {
			c.sendMsg("/timeout %v %v", usr, "30")
		}
		return
	}

	if urlMatch.MatchString(msgText) {
		if !vipMatch.MatchString(flags) {
			c.timeout(usr)
			return
		}
		return
	}

	if !russianState {
		if charMatch.MatchString(msgText) {
			c.timeout(usr)
			return
		}

		if otherLangMatch.MatchString(msgText) {
			if !timezone.MatchString(msgText) {
				c.timeout(usr)
			}
			return
		}
	}

	if lenMatch.MatchString(msgText) {
		c.timeout(usr)
		return
	}

	if tosMatch.MatchString(msgText) {
		c.timeout(usr)
		return
	}

	if spamMatch.MatchString(msgText) {
		c.timeout(usr)
		return
	}
	if botCharMatch.MatchString(msgText) || rBotStringMatch.MatchString(msgText) {
		c.sendMsg("/ban %v", usr)
		return
	}

	if ballMatch.MatchString(msgText) || (ballMatch.MatchString(msgText) && modMatch.MatchString(msgText)) {
		c.sendMsg("@%v %v", usr, ballSlice[rand.Intn(len(ballSlice))])
		return
	}

	if dungeonMatch.MatchString(msgText) {
		c.sendMsg("/timeout %v 300", usr)
		c.sendMsg("/me %v has entered the dungeon VaN", usr)
		return
	}

	if (so1.MatchString(msgText) && so4.MatchString(msgText)) || (so1.MatchString(msgText) && (so2.MatchString(msgText) || so3.MatchString(msgText))) {
		c.timeout(usr)
		return
	}

	if so1.MatchString(msgText) && so5.MatchString(msgText) {
		c.timeout(usr)
		return
	}

	if (so6.MatchString(msgText) || so7.MatchString(msgText)) && so8.MatchString(msgText) {
		c.timeout(usr)
		return
	}

	if ((ce1.MatchString(msgText) || ce2.MatchString(msgText)) && ce3.MatchString(msgText)) || ce4.MatchString(msgText) {
		c.timeout(usr)
		return
	}

	if merch.MatchString(msgText) {
		if time.Now().Sub(merchLastTime).Seconds() >= 20 {
			c.sendMsg("MyWheats: %v", mywheats)
			c.sendMsg("StreamLabs: %v", slmerch)
			c.sendMsg("TaoBao: %v", taobao)
			merchLastTime = time.Now()
			return
		}
		return

	}

	if social.MatchString(msgText) {
		if time.Now().Sub(socialLastTime).Seconds() >= 20 {
			c.sendMsg("BiliBili: %v", bilibili)
			c.sendMsg("Instagram: %v", instagram)
			c.sendMsg("Twitter: %v", twitter)
			c.sendMsg("YouTube %v", youtube)
			socialLastTime = time.Now()
			return
		}
		return
	}

	if shoutout.MatchString(msgText) {
		c.sendMsg("Want a short shoutout? Support the channel at https://streamlabs.com/vansamaofficial/tip BillyApprove")
		c.sendMsg("Want a personalized shoutout from the Dungeon Master himself? Go to https://www.cameo.com/vansamaofficial BillyApprove")
		return
	}

	if shoutoutru.MatchString(msgText) {
		c.sendMsg("Хотите короткое приветствие или ответ на вопрос от Вана? Поддержите канал донатом по ссылке https://streamlabs.com/vansamaofficial/tip BillyApprove")
		c.sendMsg("Хотите персональное видео-обращение или поздравление от самого Данжен Мастера? Переходите по https://www.cameo.com/vansamaofficial BillyApprove")
		return
	}
}

// Manages connection to twitch IRC and backs off by a factor of 2
func (c *Connection) connect() {

	holdoff := time.Second * 2
	for {
		conn, err := tls.Dial("tcp", "irc.chat.twitch.tv:6697", nil)
		if err == nil {
			c.conn = conn
			break
		}
		time.Sleep(holdoff)
		holdoff *= 2
	}
	c.sendData("PASS oauth:" + getOauth())
	c.sendData("NICK your___m0m")
}

func (c *Connection) disconnect() {
	c.sendData("QUIT Bye")
	c.conn.Close()
}

// Retrieves the Oauth token from another file
func getOauth() string {
	token, err := ioutil.ReadFile(driveName) //driveName should be the location of the file containing the Oathtoken
	if err != nil {
		panic(err)
	}
	tokenStr := string(token)
	tokenStr = strings.Trim(tokenStr, "\n")
	return tokenStr
}

func getFlags(msg string) string {
	flags := flagMatch.FindStringSubmatch(msg)
	if flags != nil {
		return flags[0]
	}
	return ""
}

// Extracts a twitch user's message from an IRC message
func getText(msg string) string {
	text := msgMatch.FindStringSubmatch(msg)
	if text != nil {
		return text[1]
	}
	return ""
}

// Extracts a twitch user's username from an IRC message
func getUser(msg string) string {
	user := userMatch.FindStringSubmatch(msg)
	if user != nil {
		return user[1]
	}
	return ""
}

// Responds to server ping
func (c *Connection) pong() {
	c.sendData("PONG")
}

// Sends a complete message to the IRC server
func (c *Connection) sendData(message string) {
	fmt.Fprintf(c.conn, "%s\r\n", message)
}

// Generic function to send a given formatted string in chat
func (c *Connection) sendMsg(format string, a ...interface{}) {
	c.sendData(fmt.Sprintf("PRIVMSG #vansamaofficial :" + fmt.Sprintf(format, a...)))
	return
}

func getChatter(user string) *Chatter {
	userListMutex.RLock()
	chatter, inMap := userList[user]
	userListMutex.RUnlock()
	if !inMap && !nukeState {
		userListMutex.Lock()
		chatter, inMap = userList[user]
		if !inMap {
			chatter = &Chatter{name: user, time: time.Now(), banDur: 5, banCt: 0}
			userList[user] = chatter
		}
		userListMutex.Unlock()
	}
	return chatter
}

// Sends the message to timeout a user
func (c *Connection) timeout(user string) {
	chatter := getChatter(user)
	if chatter.banCt == 1 {
		chatter.banDur = 30
	}
	if chatter.banCt == 2 {
		chatter.banDur = 300
	}
	c.sendMsg("/timeout %v %v", user, chatter.banDur)
	chatter.banCt += 1
	return
}

func (c *Connection) timer() {
	ticker := time.NewTicker(17 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if mediaState {
					c.sendMsg("/me Hey chat, Van will play your gachimuchi remixes if you include them in your donation! HandsUp")
				}
			case <-quit:
				ticker.Stop()
				return
			}

		}
	}()
}

func init() {
	go func() {
		for {
			func() {
				userListMutex.Lock()
				defer userListMutex.Unlock()
				for _, v := range userList {
					if time.Now().Sub(v.time) >= time.Second*300 {
						v.banDur = 5
					}
				}
			}()
			time.Sleep(time.Second * 20)
		}
	}()
}

func main() {

	c := &Connection{}
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		c.disconnect()
	}()

	c.connect()
	c.sendData("CAP REQ : twitch.tv/tags")
	c.sendData("JOIN #vansamaofficial")
	c.sendMsg("FUCK YOU vanFU")
	chat := textproto.NewReader(bufio.NewReader(c.conn))
	log, err := os.Create(logFile)
	if err != nil {
		panic(err)
	}
	defer log.Close()
	c.timer()

	for {
		msg, err := chat.ReadLine()
		if err != nil {
			panic(err)
		}
		_, err = io.WriteString(log, (msg + "\n"))
		if err != nil {
			panic(err)
		}

		fmt.Printf("> %v\n", msg)
		flags := getFlags(msg)
		usr := getUser(msg)
		msgText := getText(msg)
		c.chatMod(flags, usr, msgText)

		if strings.HasPrefix(msg, "PING") {
			c.pong()
		}

	}

}
