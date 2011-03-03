include $(GOROOT)/src/Make.inc

TARG=bin/sniffy
GOFILES=\
	alerts.go\
	db.go\
	dns.go\
	dummy.go\
	cli.go\
	events.go\
	log.go\
	main.go\
	proxy.go\
	tmpl.go\
	web/web.go\
	web/auditor.go\
	web/proxy.go\

DEPS=\
	../acl\
	../common\
	../common/queue\
	../dummy\
	../proxy\
	../sniff\

include $(GOROOT)/src/Make.cmd
