package cardhopper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/nvx/go-rfid"
	"github.com/nvx/go-rfid/pm3"
	"github.com/nvx/go-rfid/type4"
	"io"
	"log/slog"
	"time"
)

type CardHopper struct {
	writer io.Writer
	reader *bufio.Reader

	type4 *type4.Emulator

	chainingBuf []byte
}

func New(port io.ReadWriter, type4Card *type4.Emulator) *CardHopper {
	return &CardHopper{
		writer: port,
		reader: bufio.NewReader(port),
		type4:  type4Card,
	}
}

func (e *CardHopper) Close() (err error) {
	_, _ = MagicRestart.WriteToIgnoreAck(e.writer)
	time.Sleep(100 * time.Millisecond)
	_, _ = MagicRestart.WriteToIgnoreAck(e.writer)
	time.Sleep(100 * time.Millisecond)
	_, _ = MagicEnd.WriteToIgnoreAck(e.writer)

	return nil
}

func (e *CardHopper) write(ctx context.Context, packet Packet) (err error) {
	defer rfid.DeferWrap(ctx, &err)

	_, err = packet.WriteTo(e.writer, e.reader)
	return
}

func (e *CardHopper) Setup(ctx context.Context) (err error) {
	defer rfid.DeferWrap(ctx, &err)

	if reset, ok := e.writer.(interface{ ResetInputBuffer() error }); ok {
		slog.DebugContext(ctx, "Resetting port input buffer")
		err = reset.ResetInputBuffer()
		if err != nil {
			return
		}
	}

	bufLen := e.reader.Buffered()
	if bufLen > 0 {
		slog.DebugContext(ctx, "Clearing read buffer")
		_, _ = e.reader.Discard(bufLen)
	}

	slog.DebugContext(ctx, "Entering standalone mode")

	_, err = pm3.CommandEnterStandalone.WriteTo(e.writer)
	if err != nil {
		err = fmt.Errorf("error entering standalone mode: %w", err)
		return
	}

	time.Sleep(time.Second)

	slog.DebugContext(ctx, "Switching CardHopper to card mode")
	err = e.write(ctx, MagicCard)
	if err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond)

	slog.DebugContext(ctx, "Sending tag type")
	// Tag Type 11 / 0x0B
	err = e.write(ctx, Packet{11})
	if err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond)

	slog.DebugContext(ctx, "Sending timing modes")
	// Time Mode: FWI, SFGI
	err = e.write(ctx, Packet{0x0E, 0x0B})
	if err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond)

	slog.DebugContext(ctx, "Sending UID")
	// UID
	err = e.write(ctx, e.type4.UID)
	if err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond)

	slog.DebugContext(ctx, "Sending ATS")
	// ATS
	err = e.write(ctx, e.type4.ATS)
	if err != nil {
		return
	}

	return nil
}

func (e *CardHopper) Emulate(ctx context.Context) (err error) {
	defer rfid.DeferWrap(ctx, &err)

	packet := make(Packet, 0, 255)

	outPacket := make(Packet, 1, 255)

	var cid uint8 = 0xFF
	for ctx.Err() == nil {
		var n int64
		n, err = packet.ReadFrom(e.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			return
		}
		if n == 0 {
			continue
		}

		if len(packet) == 0 {
			slog.WarnContext(ctx, "Got empty packet from cardhopper standalone!")
			err = e.write(ctx, Packet{})
			if err != nil {
				return
			}
			continue
		}

		slog.DebugContext(ctx, "Got cardhopper packet", rfid.LogHex("packet", packet))

		if packet[0] == 0xE0 && len(packet) == 2 {
			cid = packet[1] & 0x0F
			fsdi := packet[1] & 0xF0 >> 4
			outPacket[0] |= 0x01 // set block number to 1
			slog.DebugContext(ctx, "Got RATS", slog.Int("cid", int(cid)), slog.Int("fsdi", int(fsdi)))
			// no need to reply to RATS
			// err = e.write(ctx, Packet{})
			//if err != nil {
			//	return
			//}
			continue
		}

		pcb := packet[0]
		hasCID := pcb&0x08 != 0

		if hasCID && len(packet) >= 2 {
			// has CID
			if packet[1]&0x0F != cid {
				slog.WarnContext(ctx, "Ignoring packet for other CID", slog.Int("cid", int(packet[1]&0x0F)), rfid.LogHex("packet", packet))
				err = e.write(ctx, Packet{})
				if err != nil {
					return
				}
				continue
			}
		}

		switch pcb & 0xC0 {
		case 0x80: // R-Block
			if pcb&0xE6 != 0xA2 {
				slog.WarnContext(ctx, "R-Block with unexpected bits set", rfid.LogHex("packet", packet))
			}
			if pcb&0x01 == outPacket[0]&0x01 {
				// Rule 11. When an R(ACK) or an R(NAK) block is received, if its block number is equal to the
				// PICC’s current block number, the last block shall be re-transmitted.
				slog.WarnContext(ctx, "R-Block triggering retransmit of last block", rfid.LogHex("packet", packet))
				err = e.write(ctx, outPacket)
				if err != nil {
					return
				}
				continue
			}

			if pcb&0x10 == 0x10 {
				// Rule 12. When an R(NAK) block is received, if its block number is not equal to the
				// PICC’s current block number, an R(ACK) block shall be sent.
				outPacket = outPacket[:len(packet)]
				copy(outPacket, packet)

				// Toggle block number
				outPacket[0] ^= 0x01

				err = e.write(ctx, outPacket)
				if err != nil {
					return
				}
				continue
			}

			slog.WarnContext(ctx, "Unexpected R-Block chaining ACK?", rfid.LogHex("packet", packet))
			err = e.write(ctx, Packet{})
			if err != nil {
				return
			}
			continue
		case 0xC0: // S-Block
			if pcb&0xC7 != 0xC2 {
				slog.WarnContext(ctx, "S-Block with unexpected bits set", rfid.LogHex("packet", packet))
			}
			if pcb&0x30 == 0 {
				// DESELECT
				slog.DebugContext(ctx, "DESELECT")
				cid = 0xFF
				e.type4.Reset(ctx)
				// Send packet back to acknowledge
				err = e.write(ctx, packet)
				if err != nil {
					return
				}
				continue
			}

			// WTX (not even forwarded by cardhopper) or RFU?
			slog.WarnContext(ctx, "Ignoring unexpected S-Block", rfid.LogHex("packet", packet))
			err = e.write(ctx, Packet{})
			if err != nil {
				return
			}
			continue

		case 0x00: // I-Block
			if pcb&0xE2 != 0x02 {
				slog.WarnContext(ctx, "I-Block with unexpected bits set", rfid.LogHex("packet", packet))
			}
		default:
			slog.WarnContext(ctx, "Bad PCB", rfid.LogHex("packet", packet))
			err = e.write(ctx, Packet{})
			if err != nil {
				return
			}
			continue
		}

		hasNAD := pcb&0x04 != 0
		chaining := pcb&0x10 != 0

		headerLen := 1
		if hasNAD {
			headerLen++
		}
		if hasCID {
			headerLen++
		}

		// Expect at least 1 INF byte after the header
		if len(packet) < headerLen+1 {
			slog.WarnContext(ctx, "Truncated packet?", rfid.LogHex("packet", packet))
			err = e.write(ctx, Packet{})
			if err != nil {
				return
			}
			continue
		}

		data := packet[headerLen:]

		if chaining {
			if hasNAD {
				slog.WarnContext(ctx, "NAD not supported with chaining", rfid.LogHex("packet", packet))
			}

			e.chainingBuf = append(e.chainingBuf, data...)

			// copy CID and block number flags from PCB for R(ACK)
			packet[0] = 0xA2 | (pcb & 0x09)
			if hasCID {
				packet = packet[:2]
			} else {
				packet = packet[:1]
			}

			err = e.write(ctx, packet)
			if err != nil {
				return
			}
			continue
		} else if len(e.chainingBuf) > 0 {
			data = append(e.chainingBuf, data...)
			e.chainingBuf = nil
		}

		// Prepare reply
		outPacket.Reset()
		// Copy PCB [CID] [NAD] bytes
		_, _ = outPacket.Write(packet[0:headerLen])

		var rapdu []byte
		rapdu, err = e.type4.Exchange(ctx, data)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			slog.WarnContext(ctx, "Failed to process APDU", rfid.ErrorAttrs(err), rfid.LogHex("apdu", data))

			// 6F00 Internal Exception
			err = e.sendReply(ctx, &outPacket, []byte{0x6F, 0x00})
			if err != nil {
				return
			}

			continue
		}

		err = e.sendReply(ctx, &outPacket, rapdu)
		if err != nil {
			return
		}
	}

	return nil
}

func (e *CardHopper) sendReply(ctx context.Context, packet *Packet, rapdu []byte) (err error) {
	defer rfid.DeferWrap(ctx, &err)

	// TODO: Handle send chaining?
	slog.DebugContext(ctx, "Sending rAPDU", rfid.LogHex("rapdu", rapdu))

	_, err = packet.Write(rapdu)
	if err != nil {
		return
	}

	err = e.write(ctx, *packet)
	if err != nil {
		return
	}

	return nil
}
