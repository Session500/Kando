package phonesync

import "rsc.io/qr"

// QRScale is how many pixels of QRCode's picture each module of the code takes.
const QRScale = 6

// QRCode is text as a QR code, a PNG of black modules on white with the four modules of white the code needs around it, QRScale pixels to a module, and how many modules it is across, that white included. It uses error correction level M, which a phone's camera reads off a screen.
func QRCode(text string) (png []byte, modules int, err error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil, 0, err
	}
	code.Scale = QRScale
	return code.PNG(), code.Size + 8, nil
}
