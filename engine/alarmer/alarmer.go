package alarmer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
)

var alarmer *Alarmer

type Alarmer struct {
	Config          *config.AlarmConfig
	alarmEmailQueue chan *Email
	alarmSmsQueue   chan *Sms
}

func NewAlarmer(config *config.AlarmConfig) *Alarmer {
	this := new(Alarmer)
	this.Config = config
	this.alarmEmailQueue = make(chan *Email)
	this.alarmSmsQueue = make(chan *Sms)
	return this
}

func (this *Alarmer) EnqueueEmail(email *Email) {
	this.alarmEmailQueue <- email
}

func (this *Alarmer) EnqueueSms(sms *Sms) {
	this.alarmSmsQueue <- sms
}

func (this *Alarmer) sendEmailAlarm(email *Email) error {
	user := this.Config.Email.User
	password := this.Config.Email.Pwd
	server := this.Config.Email.Server

	hp := strings.Split(server, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])
	content_type := "Content-Type: text/html; charset=UTF-8"

	msg := []byte("To: " + strings.Join(this.Config.Email.Notifiers, ";") + "\r\nFrom: " + user + ">\r\nSubject: " + email.subject + "\r\n" + content_type + "\r\n\r\n" + email.body)
	err := smtp.SendMail(server, auth, user, this.Config.Email.Notifiers, msg)
	if err != nil {
		log.Warn(err)
	}
	return err
}

func (this *Alarmer) sendSmsAlarm(sms *Sms) error {
	for _, notifier := range this.Config.Sms.Notifiers {
		resp, err := http.PostForm(this.Config.Sms.Gateway,
			url.Values{"templateId": {strconv.Itoa(this.Config.Sms.TemplateId)}, "deviceList": {fmt.Sprintf("[%s]", notifier)},
				"deviceType": {strconv.Itoa(0)}, "argsList": {fmt.Sprintf("[[\"%s\"]]", sms.body)},
				"contentType": {strconv.Itoa(0)}, "validTime": {strconv.FormatInt(time.Now().Add(time.Hour*8760).Unix(), 10)}})

		if err != nil {
			log.Error("Send sms error: %s", err.Error())
		}
		if resp != nil {
			defer resp.Body.Close()
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error(err.Error())
		}
		log.Debug(string(body))
	}
	return nil
}

func (this *Alarmer) Serv() {
	go func() {
		for {
			select {
			case body := <-this.alarmEmailQueue:
				this.sendEmailAlarm(body)

			case body := <-this.alarmSmsQueue:
				this.sendSmsAlarm(body)
			}
		}
	}()
}

type Email struct {
	subject string
	body    string
}

func NewEmail(subject, body string) *Email {
	return &Email{subject, body}
}

type Sms struct {
	body string
}

func NewSms(body string) *Sms {
	return &Sms{body}
}
