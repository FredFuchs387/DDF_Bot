package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/textproto"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

//Connection type allows for easier joining the twitch IRC server as well as maintaining the connection
type Connection struct {
	conn net.Conn
}

//Chatter type contains username, the time at which they were last timed out, and the duration of that timeout
type Chatter struct {
	name   string
	time   time.Time
	banDur int
}

var userList = map[string]*Chatter{}

var userMatch = regexp.MustCompile("^:([^!]+)!")
var msgMatch = regexp.MustCompile("PRIVMSG #vansamaofficial :(.*)$")
var charMatch = regexp.MustCompile("[Ѐ-ӿ]+")
var lenMatch = regexp.MustCompile("^.{400,}$")
var urlMatch = regexp.MustCompile(`http(s?)://`)

//tosSlice contains strings which violate/risk violating Twitch TOS
var tosSlice = []string{
	wordMatcher(`fag`),
	`(?i)(\W|^)n\W*i\W*(g\W*)+(e\W*|y\W*)?r`,
	`(?i)(\W|^)n\W*(i\W*|y\W*)(g\W*)+(\W|$|a)`,
	`(?i)p\W*i\W*d\W*(o\W*|a\W*)r\W*`,
	`(?i)п\W*(и\W*|й\W*)д\W*(о\W*|а\W*)р\W*`,
	`(?i)н\W*(и\W*|й\W*)г\W*(е\W*|а\W*)р`,
	wordMatcher(`retard`),
	wordMatcher(`tranny`),
}
var tosMatch = regexp.MustCompile("(?:(?:" + strings.Join(tosSlice, ")|(?:") + "))")

//engSlice contains English language strings which are to be filtered
var engSlice = []string{
	wordMatcher(`anus`),
	wordMatcherEndL(`anal`),
	`(?i)(\W|^)c\W*u+\W*m+\W*($|\s)`,
	wordMatcher(`fisting`),
	wordMatcher(`gloryhole`),
	wordMatcher(`penis`),
	wordMatcher(`semen`),
}
var engMatch = regexp.MustCompile("(?:(?:" + strings.Join(engSlice, ")|(?:") + "))")

//otherLangSlice contains non English strings which are to be filtered
var otherLangSlice = []string{
	wordMatcherEndL(`dela`),
	wordMatcherEndL(`ebani`),
	wordMatcherEndL(`ebat`),
	wordMatcherEndL(`eto`),
	wordMatcherEndL(`kak`),
	wordMatcher(`kogda`),
	wordMatcher(`meste`),
	wordMatcher(`pizdec`),
	wordMatcher(`vpered`),
	wordMatcher(`vperde`),
	wordMatcher(`vsem`),
	wordMatcherEndL(`za`),
	`(?i)z\W*d\W*a\W*r\W*o\W*(v\W*|w\W*)a`,
}
var otherLangMatch = regexp.MustCompile("(?:(?:" + strings.Join(otherLangSlice, ")|(?:") + "))")

//spamSlice contains strings which are deemed to be spam
var spamSlice = []string{
	wordMatcherEndL(`stray228`),
	wordMatcherEndL(`wewe`),
	wordMatcher(`flexair`),
}
var spamMatch = regexp.MustCompile("(?:(?:" + strings.Join(spamSlice, ")|(?:") + "))")

//linkSlice contains approved sites to be posted in chat
var linkSlice = []string{
	`http(s?)://(?:www\.)?clips.twitch.tv/\.*`,
	`http(s?)://(?:www\.)?twitch.tv/\.*`,
	`http(s?)://(?:www\.)?youtube.com/\.*`,
	`http(s?)://(?:www\.)?youtu.be/\.*`,
	`http(s?)://(?:www\.)?discord.gg/\.*`,
	`http(s?)://(?:www\.)?streamlabs.com/\.*`,
	`http(s?)://(?:www\.)?cameo.com/vansamaofficial`,
	`http(s?)://(?:www\.)?space.bilibili.com/\.*`,
	`http(s?)://(?:www\.)?gofundme.com/\.*`,
	`http(s?)://shop170176806.world.taobao.com/\.*`,
}

var linkMatch = regexp.MustCompile("(?:(?:" + strings.Join(linkSlice, ")|(?:") + "))")

//Converts a normal string into a consistent regex pattern
func wordMatcher(word string) string {
	return `(?i)(\W|^)` + strings.Join(strings.Split(word, ""), `+\W*`)
}

//As wordMatcher, except this pattern checks for trailing non-words or endline
func wordMatcherEndL(word string) string {
	return `(?i)(\W|^)` + strings.Join(strings.Split(word, ""), `\W*`) + `(\W|$)`
}

//Checks text extracted from IRC against the blacklisted regex and notifies the offender
func (c *Connection) chatMod(usr string, msgText string) {
	if lenMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v Don't Spam Chat MODS", usr)
		return
	}

	if tosMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v Against TOS MODS", usr)
		return
	}

	if engMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v Excessive Vulgarity MODS", usr)
		return
	}

	if otherLangMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v English Language Only MODS", usr)
		return
	}

	if charMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v English Language Only MODS", usr)
		return
	}

	if spamMatch.MatchString(msgText) {
		c.timeout(usr)
		c.sendMsg("@%v Don't Spam Chat MODS", usr)
		return
	}
	if urlMatch.MatchString(msgText) {
		if !linkMatch.MatchString(msgText) {
			c.timeout(usr)
			c.sendMsg("@%v Don't Post Random Links MODS", usr)
			return
		}
		return
	}
}

//Manages connection to twitch IRC and backs off by a factor of 2
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
	c.sendData("NICK ddf_bot")
}

func (c *Connection) disconnect() {
	c.sendData("QUIT Bye")
	c.conn.Close()
}

//Retrieves the Oauth token from another file
func getOauth() string {
	token, err := ioutil.ReadFile(driveName) //driveName should be the location of the file containing the Oathtoken
	if err != nil {
		panic(err)
	}
	tokenStr := string(token)
	tokenStr = strings.Trim(tokenStr, "\n")
	return tokenStr
}

//Extracts a twitch user's message from an IRC message
func getText(msg string) string {
	text := msgMatch.FindStringSubmatch(msg)
	if text != nil {
		return text[1]
	}
	return ""
}

//Extracts a twitch user's username from an IRC message
func getUser(msg string) string {
	user := userMatch.FindStringSubmatch(msg)
	if user != nil {
		return user[1]
	}
	return ""
}

//Responds to server ping
func (c *Connection) pong() {
	c.sendData("PONG")
}

//Sends a complete message to the IRC server
func (c *Connection) sendData(message string) {
	fmt.Fprintf(c.conn, "%s\r\n", message)
}

//Generic function to send a given formatted string in chat
func (c *Connection) sendMsg(format string, a ...interface{}) {
	c.sendData(fmt.Sprintf("PRIVMSG #vansamaofficial :" + fmt.Sprintf(format, a...)))
	return
}

//Sends the message to timeout a user
func (c *Connection) timeout(user string) {
	chatter, inMap := userList[user]
	if !inMap {
		chatter = &Chatter{name: user, time: time.Now(), banDur: 6}
		userList[user] = chatter
	}
	c.sendMsg("/timeout %v %v", user, chatter.banDur)
	chatter.banDur *= 50
	return
}

//Ranges through the userList and changes a chatter's banDur back to 6 if it has been 2 minutes or longer since their last timeout
func init() {
	go func() {
		for {
			for _, v := range userList {
				if time.Now().Sub(v.time) >= time.Second*120 {
					v.banDur = 6
				}
			}
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
	c.sendData("JOIN #vansamaofficial")
	c.sendMsg("FUCK YOU vanFU")
	chat := textproto.NewReader(bufio.NewReader(c.conn))

	for {
		msg, err := chat.ReadLine()
		if err != nil {
			panic(err)
		}
		fmt.Printf("> %v\n", msg)
		usr := getUser(msg)
		msgText := getText(msg)

		c.chatMod(usr, msgText)

		if strings.HasPrefix(msg, "PING") {
			c.pong()
		}

	}

}
