module webtyp.com/view

go 1.26.8

require (
	webtyp.com/dom v0.13.29
	webtyp.com/fmt v1.0.0
	webtyp.com/input v0.0.36
	webtyp.com/json v0.5.40
	webtyp.com/lang v0.1.14
	webtyp.com/mcp v0.3.14
	webtyp.com/model v0.2.13
	webtyp.com/router v0.4.17
)

require (
	webtyp.com/base64 v0.0.6 // indirect
	webtyp.com/calendar v0.1.0 // indirect
	webtyp.com/context v0.0.23 // indirect
	webtyp.com/crud v0.0.2 // indirect
	webtyp.com/escape v0.1.0 // indirect
	webtyp.com/fetch v0.1.29 // indirect
	webtyp.com/filepath v0.1.0 // indirect
	webtyp.com/time v0.6.4 // indirect
	webtyp.com/unixid v0.3.1 // indirect
)

replace webtyp.com/model => ../model

replace webtyp.com/router => ../router

replace webtyp.com/crud => ../crud

replace webtyp.com/rpc => ../rpc

replace webtyp.com/mcp => ../mcp
