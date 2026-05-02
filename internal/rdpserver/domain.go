package rdpserver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

const (
	mcsErectDomainRequestApp = 1
	mcsAttachUserRequestApp  = 10
	mcsAttachUserConfirmApp  = 11
	mcsChannelJoinRequestApp = 14
	mcsChannelJoinConfirmApp = 15
	defaultMCSUserID         = 1001
	domainReadTimeout        = 10 * time.Second
)

type domainPDU struct {
	Application int
	Initiator   uint16
	ChannelID   uint16
	Data        []byte
}

func handleMCSDomainSequence(conn net.Conn, frames frame.Source, sink input.Sink, width, height int, auth Authenticator) error {
	userID := uint16(defaultMCSUserID)
	for i := 0; i < 32; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(domainReadTimeout))
		pdu, err := readMCSDomainPDU(conn)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil
			}
			return err
		}

		tracef("mcs_domain_pdu", "application=%d initiator=%d channel=%d data_len=%d", pdu.Application, pdu.Initiator, pdu.ChannelID, len(pdu.Data))
		switch pdu.Application {
		case mcsErectDomainRequestApp:
			// No response for ErectDomainRequest.
		case mcsAttachUserRequestApp:
			if err := writeMCSAttachUserConfirm(conn, userID); err != nil {
				return err
			}
		case mcsChannelJoinRequestApp:
			if err := writeMCSChannelJoinConfirm(conn, pdu.Initiator, pdu.ChannelID); err != nil {
				return err
			}
		case mcsSendDataRequestApp:
			if share, err := parseShareControlPDU(pdu.Data); err == nil {
				switch share.PDUType {
				case pduTypeConfirmActive:
					if _, err := parseConfirmActive(pdu.Data); err != nil {
						return err
					}
				case pduTypeData:
					if err := handleShareDataPDU(conn, share, frames, sink, width, height); err != nil {
						return err
					}
				}
				continue
			}
			sec, err := parseSecurityPDU(pdu.Data)
			if err != nil {
				return err
			}
			if sec.Flags&secInfoPacket != 0 {
				clientInfo, err := parseClientInfo(sec.Payload)
				if err != nil {
					if auth != nil {
						return err
					}
					tracef("client_info_parse", "err=%v", err)
				} else {
					tracef("client_info", "user=%q domain=%q flags=0x%08x", clientInfo.UserName, clientInfo.Domain, clientInfo.Flags)
					if err := authenticateClientInfo(auth, clientInfo); err != nil {
						return fmt.Errorf("auth failed: %w", err)
					}
				}
				if err := writeDemandActive(conn, width, height); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported MCS domain PDU application %d", pdu.Application)
		}
	}
	return nil
}

func readMCSDomainPDU(conn net.Conn) (*domainPDU, error) {
	payload, err := readTPKT(conn)
	if err != nil {
		return nil, fmt.Errorf("read tpkt: %w", err)
	}
	mcs, err := parseX224Data(payload)
	if err != nil {
		return nil, fmt.Errorf("parse x224 data: %w", err)
	}
	return parseMCSDomainPDU(mcs)
}

func parseMCSDomainPDU(data []byte) (*domainPDU, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty MCS domain PDU")
	}
	pdu := &domainPDU{Application: int(data[0] >> 2)}
	body := data[1:]
	switch pdu.Application {
	case mcsChannelJoinRequestApp:
		if len(body) < 4 {
			return nil, fmt.Errorf("short ChannelJoinRequest")
		}
		pdu.Initiator = binary.BigEndian.Uint16(body[0:2]) + defaultMCSUserID
		pdu.ChannelID = binary.BigEndian.Uint16(body[2:4])
	case mcsSendDataRequestApp, mcsSendDataIndicationApp:
		req, err := parseMCSSendDataRequest(body)
		if err != nil {
			return nil, err
		}
		pdu.Initiator = req.Initiator
		pdu.ChannelID = req.ChannelID
		pdu.Data = req.Data
	}
	return pdu, nil
}

func writeMCSAttachUserConfirm(conn net.Conn, initiator uint16) error {
	tracef("mcs_attach_user_confirm", "initiator=%d", initiator)
	body := []byte{0} // result: rt-successful
	body = append(body, encodePERInteger16(initiator, defaultMCSUserID)...)
	return writeMCSDomainPDU(conn, mcsAttachUserConfirmApp, body)
}

func writeMCSChannelJoinConfirm(conn net.Conn, initiator, channelID uint16) error {
	tracef("mcs_channel_join_confirm", "initiator=%d channel=%d", initiator, channelID)
	body := []byte{0} // result: rt-successful
	body = append(body, encodePERInteger16(initiator, defaultMCSUserID)...)
	body = append(body, encodePERInteger16(channelID, 0)...)
	body = append(body, encodePERInteger16(channelID, 0)...)
	return writeMCSDomainPDU(conn, mcsChannelJoinConfirmApp, body)
}

func writeMCSDomainPDU(conn net.Conn, application int, body []byte) error {
	mcs := append([]byte{byte(application << 2)}, body...)
	x224 := append([]byte{0x02, x224TypeData, 0x80}, mcs...)
	return writeTPKT(conn, x224)
}

func encodePERInteger16(value, minimum uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, value-minimum)
	return buf
}
