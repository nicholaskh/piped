package config

import (
	conf "github.com/nicholaskh/jsconf"
)

type AlarmConfig struct {
	Email *EmailConfig
	Sms   *SmsConfig
}

func (this *AlarmConfig) LoadConfig(cf *conf.Conf) {
	this.Email = new(EmailConfig)
	section, err := cf.Section("email")
	if err != nil {
		panic(err)
	}
	this.Email.LoadConfig(section)

	this.Sms = new(SmsConfig)
	section, err = cf.Section("sms")
	if err != nil {
		panic(err)
	}
	this.Sms.LoadConfig(section)
}

type EmailConfig struct {
	Server    string
	User      string
	Pwd       string
	Notifiers []string
}

func (this *EmailConfig) LoadConfig(cf *conf.Conf) {
	this.Server = cf.String("server", "")
	this.User = cf.String("user", "")
	this.Pwd = cf.String("pwd", "")
	this.Notifiers = cf.StringList("notifiers", nil)

	if this.Server == "" ||
		this.User == "" ||
		this.Pwd == "" ||
		this.Notifiers == nil {
		panic("Imcomplete email alarm config")
	}
}

type SmsConfig struct {
}

func (this *SmsConfig) LoadConfig(cf *conf.Conf) {

}
