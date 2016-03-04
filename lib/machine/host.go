package machine

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/docker"
	"github.com/jeffjen/machine/lib/ssh"

	"bytes"
	"fmt"
	"time"
)

type Host struct {
	CertPath     string
	Organization string

	User     string
	Cert     string
	IsDocker bool

	// SSH config for command forwarding
	cmdr ssh.Commander
}

func (h *Host) InstallDockerEngineCertificate(host string, altname ...string) error {
	if !h.IsDocker { // Not processing because not a Docker Engine
		fmt.Println(host, "- skipping Docker Certificate Install")
		return nil
	}
	ssh_config := ssh.Config{User: h.User, Server: host, Key: h.Cert, Port: "22"}
	h.cmdr = ssh.New(ssh_config)

	var subAltNames = []string{
		host,
		"localhost",
		"127.0.0.1",
	}
	subAltNames = append(subAltNames, altname...)

	fmt.Println(host, "- generate cert for subjects -", subAltNames)
	CA, Cert, Key, err := cert.GenerateServerCertificate(h.CertPath, h.Organization, subAltNames)
	if err != nil {
		return err
	}

	fmt.Println(host, "- configure docker engine")
	h.cmdr.Sudo()
	return h.sendEngineCertificate(CA, Cert, Key)
}

func (h *Host) sendEngineCertificate(ca, cert, key *cert.PemBlock) error {
	const attempts = 5

	// Wait for SSH daemon online
	var idx = 0
	for ; idx < attempts; idx++ {
		if _, err := h.cmdr.Run("date"); err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if idx == attempts {
		return fmt.Errorf("%s - Unable to contact remote", h.cmdr.Host())
	}

	if err := h.cmdr.Copy(cert.Buf, int64(cert.Buf.Len()), "/etc/docker/"+cert.Name, 0644); err != nil {
		return err
	}
	fmt.Println(h.cmdr.Host(), "- Cert sent")

	if err := h.cmdr.Copy(key.Buf, int64(key.Buf.Len()), "/etc/docker/"+key.Name, 0600); err != nil {
		return err
	}
	fmt.Println(h.cmdr.Host(), "- Key sent")

	if err := h.cmdr.Copy(ca.Buf, int64(ca.Buf.Len()), "/etc/docker/"+ca.Name, 0644); err != nil {
		return err
	}
	fmt.Println(h.cmdr.Host(), "- CA sent")

	if err := h.configureDockerTLS(); err != nil {
		return err
	}
	fmt.Println(h.cmdr.Host(), "- Configured Docker Engine")

	h.stopDocker()
	fmt.Println(h.cmdr.Host(), "- Stopped Docker Engine")

	if err := h.startDocker(); err != nil {
		return err
	}
	fmt.Println(h.cmdr.Host(), "- Started Docker Engine")

	return nil
}

func (h *Host) configureDockerTLS() error {
	const (
		daemonPath = "/etc/docker/daemon.json"

		CAPem   = "/etc/docker/ca.pem"
		CertPem = "/etc/docker/server-cert.pem"
		KeyPem  = "/etc/docker/server-key.pem"
	)

	var (
		dOpts *docker.DaemonConfig

		buf = new(bytes.Buffer)
	)

	err := h.cmdr.Load(daemonPath, buf)
	if err != nil {
		return err
	}
	dOpts, err = docker.LoadDaemonConfig(buf.Bytes())
	if err != nil {
		return err
	}
	dOpts.AddHost("tcp://0.0.0.0:2375")
	dOpts.TlsVerify = true
	dOpts.TlsCACert = CAPem
	dOpts.TlsCert = CertPem
	dOpts.TlsKey = KeyPem
	if r, err := dOpts.Reader(); err != nil {
		return err
	} else {
		return h.cmdr.Copy(r, int64(r.Len()), daemonPath, 0600)
	}
}

func (h *Host) startDocker() error {
	_, e := h.cmdr.Run("service docker start")
	return e
}

func (h *Host) stopDocker() error {
	_, e := h.cmdr.Run("service docker stop")
	return e
}
