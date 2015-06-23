package engine

import (
	"net/smtp"
	"strings"

	"github.com/nicholaskh/piped/config"
)

var alarmer *Alarmer

type Alarmer struct {
	config          *config.AlarmConfig
	alarmEmailQueue chan *Email
	alarmSmsQueue   chan *Sms
}

func NewAlarmer(config *config.AlarmConfig) *Alarmer {
	this := new(Alarmer)
	this.config = config
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
	user := this.config.Email.User
	password := this.config.Email.Pwd
	server := this.config.Email.Server

	hp := strings.Split(server, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])
	content_type := "Content-Type: text/html; charset=UTF-8"

	msg := []byte("To: " + strings.Join(this.config.Email.Notifiers, ";") + "\r\nFrom: " + user + ">\r\nSubject: " + email.subject + "\r\n" + content_type + "\r\n\r\n" + email.body)
	err := smtp.SendMail(server, auth, user, this.config.Email.Notifiers, msg)
	return err
}

func (this *Alarmer) sendSmsAlarm(sms *Sms) error {
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
}

func NewSms() *Sms {
	return &Sms{}
}
