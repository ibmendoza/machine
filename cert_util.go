package main

import (
	"github.com/jeffjen/machine/lib/cert"

	"github.com/codegangsta/cli"

	"fmt"
	"os"
	usr "os/user"
	path "path/filepath"
	"strings"
)

func parseCertArgs(c *cli.Context) (org, certpath string, err error) {
	user, err := usr.Current()
	if err != nil {
		return // Unable to determine user
	}
	org = c.Parent().String("organization")
	certpath = c.Parent().String("certpath")
	certpath = strings.Replace(certpath, "~", user.HomeDir, 1)
	certpath, err = path.Abs(certpath)
	if err != nil {
		return
	}
	err = os.MkdirAll(certpath, 0700)
	return
}

func generateServerCertificate(c *cli.Context) (CA, Cert, Key *cert.PemBlock) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		fmt.Println("You must provide hostname to create Certificate for")
		os.Exit(1)
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := parseCertArgs(c)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	CA, Cert, Key, err = cert.GenerateServerCertificate(certpath, org, hosts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return
}
