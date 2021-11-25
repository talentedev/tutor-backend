package store

import (
	"fmt"
	"strings"
)

// URI models connection string URI
type URI struct {
	Database    string
	Hosts       []string
	Password    string
	User        string
	SSL         bool
	Options     []string
	Certificate string
	URL         string
}

func (u *URI) String() (connString string) {
	if len(u.URL) > 0 {
		return u.URL
	}
	hosts := strings.Join(u.Hosts, ",")
	u.sslOption()
	var data = []string{
		u.protocol(),
		u.auth(),
		hosts,
		"/", u.Database,
		u.options(),
	}
	connString = strings.Join(data, "")
	return
}

func (u *URI) options() (options string) {
	if len(u.Options) > 0 {
		options = fmt.Sprintf("?%v", strings.Join(u.Options, "&"))
	}
	return
}

func (u *URI) protocol() (protocol string) {
	protocol = "mongodb://"
	if u.SSL {
		//TODO: Change protocol to protocol = "mongodb+srv://" when mongo has been updated to 3.6
		// Read more about here -> https://docs.mongodb.com/manual/reference/connection-string/#dns-seedlist-connection-format
		protocol = "mongodb://"
	}
	return
}
func (u *URI) auth() (data string) {
	data = ""
	if u.User != "" && u.Password != "" {
		data = fmt.Sprintf("%v:%v@", u.User, u.Password)
	}
	return
}

func (u *URI) sslOption() {
	var option string
	if u.SSL {
		option = "ssl=true"
		u.Options = append(u.Options, option)
	}
	return
}
