package component

import "github.com/robfig/cron/v3"

type CronWrap struct {
	Cron *cron.Cron
}

func NewCronWrap() CronWrap {
	return CronWrap{
		Cron: cron.New(cron.WithParser(cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		))),
	}
}
