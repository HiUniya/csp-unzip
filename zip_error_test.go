package main

import "testing"

func TestPasswordDiagnostic(t *testing.T) {
	for _, detail := range []string{
		"ERROR: C:\\data\\outer.7z\r\nCannot open encrypted archive. Wrong password?\r\n\r\nERRORS:\r\nHeaders Error",
		"ERRORS:\nHeaders Error in encrypted archive. Wrong password?",
		"ERROR: Wrong password : test01.in",
		"ERROR: Data Error in encrypted file. Wrong password? : test01.in",
		"ERROR: CRC Failed in encrypted file. Wrong password? : test01.in",
	} {
		if !isPasswordDiagnostic(detail) {
			t.Errorf("missed password diagnostic %q", detail)
		}
	}
	for _, detail := range []string{
		"",
		"ERROR: C:\\data\\Wrong password.7z\r\nCannot open the file as archive",
		"ERROR: Data Error : Wrong password",
		"ERROR: CRC Failed : Wrong password.txt",
		"ERRORS:\nHeaders Error",
		"ERROR: Unexpected end of data : test01.in",
		"ERROR: Write error : test01.in",
		"ERROR: Unsupported Method : test01.in",
	} {
		if isPasswordDiagnostic(detail) {
			t.Errorf("misidentified error %q as a password failure", detail)
		}
	}
}
