package wg

import (
	"akhokhlow80/tanlnode/cmd"
	"akhokhlow80/tanlnode/peerstats"
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TODO: context???

type Service struct {
	ifce        string
	wgExecPath  string
	wgNetNSPath string
}

func NewService(interfaceName string, wgPath string, wgNetNSPath string) Service {
	return Service{ifce: interfaceName, wgExecPath: wgPath, wgNetNSPath: wgNetNSPath}
}

type PeerConfig struct {
	PublicKey           wgtypes.Key
	PresharedKey        *wgtypes.Key
	Endpoint            string // optional
	PersistentKeepalive int64  // optional
	AllowedIPs          []netip.Prefix
}

func (s *Service) PutPeer(p *PeerConfig) error {
	args := []string{"set", s.ifce, "peer", p.PublicKey.String()}
	if p.PresharedKey != nil {
		tempFile, err := os.CreateTemp("", "preshared-key")
		if err != nil {
			return err
		}
		defer os.Remove(tempFile.Name())

		b64 := p.PresharedKey.String()
		for len(b64) != 0 {
			n, err := tempFile.Write([]byte(b64))
			if err != nil {
				return err
			}
			b64 = b64[n:]
		}
		if err := tempFile.Sync(); err != nil {
			return err
		}
		args = append(args, "preshared-key", tempFile.Name())
	}
	if len(p.Endpoint) != 0 {
		args = append(args, "endpoint", p.Endpoint)
	}
	if p.PersistentKeepalive != 0 {
		args = append(args, "persistent-keepalive", strconv.Itoa(int(p.PersistentKeepalive)))
	}
	if len(p.AllowedIPs) > 0 {
		var sb strings.Builder
		for i, allowedIP := range p.AllowedIPs {
			sb.WriteString(allowedIP.String())
			if i != len(p.AllowedIPs)-1 {
				sb.WriteRune(',')
			}
		}
		args = append(args, "allowed-ips", sb.String())
	}

	_, err := s.execWGCmd(true, args)
	return err
}

func (s *Service) RemovePeer(publicKey wgtypes.Key) error {
	_, err := s.execWGCmd(true, []string{"set", s.ifce, "peer", publicKey.String(), "remove"})
	return err
}

type PeerStat struct {
	PublicKey wgtypes.Key
	peerstats.Stat
}

func (s *Service) GetPeerStats() ([]PeerStat, error) {
	output, err := s.execWGCmd(false, []string{"show", s.ifce, "dump"})
	if err != nil {
		return nil, err
	}

	var peers []PeerStat

	firstLine := true
	for line := range bytes.SplitSeq(output, []byte{'\n'}) {
		// skip first line
		if firstLine {
			firstLine = false
			continue
		}
		if len(line) == 0 {
			continue
		}

		columns := bytes.Split(line, []byte{'\t'})
		if len(columns) < 8 {
			return nil, fmt.Errorf("Invalid line in wg output: `%s`", line)
		}

		pubkey := string(columns[0])
		endpoint := string(columns[2])
		latestHandshake := string(columns[4])
		transferRx := string(columns[5])
		transferTx := string(columns[6])

		parsedPubkey, err := wgtypes.ParseKey(pubkey)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to parse public key `%s` from wg: %s",
				pubkey,
				err,
			)
		}
		if endpoint == "(none)" {
			endpoint = ""
		}
		parsedLatestHandshake, err := strconv.ParseInt(latestHandshake, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to parse latest-handshake `%s` from wg: %s",
				latestHandshake,
				err,
			)
		}
		// prevent Jan 1 1970
		var parsedLatestHandshakeTime time.Time
		if parsedLatestHandshake != 0 {
			parsedLatestHandshakeTime = time.Unix(parsedLatestHandshake, 0).UTC()
		}
		parsedTransferRx, err := strconv.ParseInt(transferRx, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to parse transfer-rx `%s` from wg: %s",
				transferRx,
				err,
			)
		}
		parsedTransferTx, err := strconv.ParseInt(transferTx, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"Failed to parse transfer-tx `%s` from wg: %s",
				transferTx,
				err,
			)
		}
		peers = append(peers, PeerStat{
			PublicKey: parsedPubkey,
			Stat: peerstats.Stat{
				LatestEndpoint:  endpoint,
				LatestHandshake: parsedLatestHandshakeTime,
				Tx:              parsedTransferTx,
				Rx:              parsedTransferRx,
			},
		})
	}
	return peers, nil
}

func (s *Service) execWGCmd(verbose bool, wgArgs []string) ([]byte, error) {
	return cmd.Exec(s.wgNetNSPath, verbose, s.wgExecPath, wgArgs)
}
