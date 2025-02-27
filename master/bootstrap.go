package master

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	"github.com/alexhunt7/gofigure/credentials"
	pb "github.com/alexhunt7/gofigure/proto"
	"github.com/alexhunt7/ssher"
)

type Creds struct {
	CAFile   string
	CertFile string
	KeyFile  string
}

type MinionConfig struct {
	Bind  net.IP
	Port  int
	Creds *Creds
}

func putfile(sftpClient *sftp.Client, src, dst string, perms os.FileMode) error {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := sftpClient.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	err = sftpClient.Chmod(dst, perms)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}

	// TODO fsync?
	return nil
}

// Bootstrap will parse an openssh config file, ssh to the remote host, copy the executable there,
// run it, and attempt to connect, returning a gofigure client.
func Bootstrap(host, sshConfigPath, executable string, minionConfig *MinionConfig, masterCreds *Creds) (*Client, error) {
	var gofigureClient *Client

	sshConfig, connectString, err := ssher.ClientConfig(host, sshConfigPath)
	if err != nil {
		return gofigureClient, err
	}

	sshConn, err := ssh.Dial("tcp", connectString, sshConfig)
	if err != nil {
		return gofigureClient, err
	}
	defer sshConn.Close()

	// TODO kill existing process?

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		return gofigureClient, err
	}
	defer sftpClient.Close()

	err = putfile(sftpClient, executable, path.Base(executable), 0700)
	if err != nil {
		return gofigureClient, err
	}

	for _, filename := range []string{
		minionConfig.Creds.CAFile,
		minionConfig.Creds.CertFile,
		minionConfig.Creds.KeyFile,
	} {
		err = putfile(sftpClient, filename, path.Base(filename), 0600)
		if err != nil {
			return gofigureClient, err
		}
	}

	session, err := sshConn.NewSession()
	if err != nil {
		return gofigureClient, err
	}
	defer session.Close()

	err = session.Start(fmt.Sprintf("./%s serve --bind %s --port %d --caFile %s --certFile %s --keyFile %s </dev/null &>/dev/null",
		path.Base(executable),
		minionConfig.Bind,
		minionConfig.Port,
		path.Base(minionConfig.Creds.CAFile),
		path.Base(minionConfig.Creds.CertFile),
		path.Base(minionConfig.Creds.KeyFile),
	))
	if err != nil {
		return gofigureClient, err
	}

	splitConnectString := strings.Split(connectString, ":")
	grpcConnectString := fmt.Sprintf("%s:%d", splitConnectString[0], minionConfig.Port)

	conn, err := ConnectGRPC(grpcConnectString, masterCreds.CAFile, masterCreds.CertFile, masterCreds.KeyFile)
	if err != nil {
		return gofigureClient, err
	}

	return &Client{GofigureClient: pb.NewGofigureClient(conn)}, nil
}

// ConnectGRPC attempts to connect over GRPC to the remote address.
// It returns only a GRPC connection, not a gofigure client.
func ConnectGRPC(address, caFile, certFile, keyFile string) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	tries := 1
	maxTries := 30
	//for i := 0; i < 30; i++ {
	for {
		c, err := net.Dial("tcp", address)
		if err == nil {
			// TODO reuse this connection instead of closing it
			c.Close()
			break
		}
		tries++
		if tries > maxTries {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}

	creds, err := credentials.Load(caFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}
	for {
		conn, err = grpc.Dial(address,
			grpc.WithTransportCredentials(creds),
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
		)
		if err == nil {
			break
		}
	}
	return conn, nil
}

// BootstrapMany calls Bootstrap for many hosts in parallel.
func BootstrapMany(sshConfigPath string, executable string, minionConfigs map[string]*MinionConfig, masterCreds *Creds) (map[string]*Client, error) {
	type result struct {
		host   string
		client *Client
	}
	gobootstrap := func(host, sshConfigPath, executable string, minionConfig *MinionConfig, masterCreds *Creds, successChan chan<- *result, failChan chan<- error) {
		client, err := Bootstrap(host, sshConfigPath, executable, minionConfig, masterCreds)
		if err != nil {
			failChan <- err
			return
		}
		successChan <- &result{host: host, client: client}
	}

	successChan, failChan := make(chan *result), make(chan error)
	for host, minionConfig := range minionConfigs {
		go gobootstrap(host, sshConfigPath, executable, minionConfig, masterCreds, successChan, failChan)
	}

	clients := make(map[string]*Client)
	for range minionConfigs {
		select {
		case result := <-successChan:
			clients[result.host] = result.client
		case err := <-failChan:
			// TODO return multiple errors?
			return nil, err
		}
	}
	return clients, nil
}
