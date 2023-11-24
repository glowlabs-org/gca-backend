package main

// The bootstrap binary is a tool that will generate all of the files necessary
// to simulate a proper GCA, and then we just copy the keys to the right place.
// This isn't technically a good solution, but since the full software stack
// isn't quite complete yet it's how we brought our first farm online.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"

	"github.com/glowlabs-org/gca-backend/glow"
)

type GCAServer struct {
	Banned   bool
	Location string
	HttpPort uint16
	TcpPort  uint16
	UdpPort  uint16
}

func SerializeGCAServerMap(gcaMap map[glow.PublicKey]GCAServer) ([]byte, error) {
	var buffer bytes.Buffer

	for key, server := range gcaMap {
		// Serialize the key (PublicKey)
		if _, err := buffer.Write(key[:]); err != nil {
			return nil, err
		}

		// Serialize the 'Banned' bool as a single byte
		if server.Banned {
			buffer.WriteByte(1)
		} else {
			buffer.WriteByte(0)
		}

		// Serialize the length of 'Location' string as a uint16
		if len(server.Location) > 0xFFFF { // the max value of uint16
			return nil, fmt.Errorf("location string is too long")
		}
		locationLength := uint16(len(server.Location))
		if err := binary.Write(&buffer, binary.LittleEndian, locationLength); err != nil {
			return nil, err
		}

		// Serialize the 'Location' string
		if _, err := buffer.WriteString(server.Location); err != nil {
			return nil, err
		}

		// Serialize the ports (HttpPort, TcpPort, UdpPort) as uint16
		if err := binary.Write(&buffer, binary.LittleEndian, server.HttpPort); err != nil {
			return nil, err
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.TcpPort); err != nil {
			return nil, err
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.UdpPort); err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

func sendPendingAuth() error {
	//location = "65.21.201.235"
	//location = "95.216.9.86"
	//location = "65.109.101.89"
	//location = "157.90.212.116"
	j, err := ioutil.ReadFile("auth-me.json")
	if err != nil {
		fmt.Println(err)
		return err
	}
	resp, err := http.Post("http://65.21.201.235:35015/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(j))
	if err != nil {
		fmt.Println(err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		ra, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("NOT OKAY:", string(ra))
		return fmt.Errorf("NOT OKAY: %s", string(ra))
	}
	return nil
}

func main() {
	// Create the file if it does not exist.
	fname := "temp-gca-keys.dat"
	_, err := os.Stat(fname)
	if os.IsNotExist(err) {
		// Create a keypair for the fake GCA and save it to the file.
		pubkey, privkey := glow.GenerateKeyPair()
		f, err := os.Create(fname)
		if err != nil {
			fmt.Println(err)
			return
		}
		var data [96]byte
		copy(data[:32], pubkey[:])
		copy(data[32:], privkey[:])
		_, err = f.Write(data[:])
		if err != nil {
			fmt.Println(err)
			return
		}
		err = f.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Read the keys from the temp gca file.
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(data) != 96 {
		fmt.Println("wrong data")
		return
	}
	var gcaPub glow.PublicKey
	var gcaPriv glow.PrivateKey
	copy(gcaPub[:], data[:32])
	copy(gcaPriv[:], data[32:])

	/* - this code is commented out because now that we've run it, we want
	* to freeze it.
	// Create server keys for 4 servers, and create the server file that
	// the device will use to connect to all the servers.
	m := make(map[glow.PublicKey]GCAServer)
	for i := 0; i < 4; i++ {
		var location string
		if i == 0 {
			location = "65.21.201.235"
		} else if i == 2 {
			location = "95.216.9.86"
		} else if i == 3 {
			location = "65.109.101.89"
		} else {
			location = "157.90.212.116"
		}

		// Create keys for a gca server
		gcasPub, gcasPriv := glow.GenerateKeyPair()
		copy(data[:32], gcasPub[:])
		copy(data[32:], gcasPriv[:])
		err = ioutil.WriteFile(fmt.Sprintf("server.keys.%v", location), data, 0644)
		if err != nil {
			fmt.Println("Unable to create server keys")
			return
		}
		m[gcasPub] = GCAServer {
			Banned: false,
			Location: location,
			HttpPort: 35015,
			TcpPort: 35030,
			UdpPort: 35045,
		}

	}
	data, err = SerializeGCAServerMap(m)
	if err != nil {
		fmt.Println("unable to serialize")
		return
	}
	err = ioutil.WriteFile("gca-servers.dat", data, 0644)
	if err != nil {
		fmt.Println("gca servers write:", err)
	}

	// Create the gca key file.
	err = ioutil.WriteFile("gca.pubkey", gcaPub[:], 0644)
	if err != nil {
		fmt.Println("unable to write gca pubkey")
		return
	}
	// Use the same gca key as the tempkey. This is just so startup goes
	// smoothly on the first try.
	err = ioutil.WriteFile("gca.tempkey", gcaPub[:], 0644)
	if err != nil {
		fmt.Println("unable to write gca pubkey")
		return
	}
	*/

	// Create keys for a device.
	devPub, devPriv := glow.GenerateKeyPair()
	copy(data[:32], devPub[:])
	copy(data[32:], devPriv[:])
	err = ioutil.WriteFile("client.keys", data[:96], 0644)
	if err != nil {
		fmt.Println("unable to write gca pubkey")
		return
	}

	// Create an authorization for the equipment
	sid := rand.Intn(400) + 15
	ea := glow.EquipmentAuthorization{
		ShortID:   uint32(sid),
		PublicKey: devPub,
		Capacity:  2520e3,   // milliwatt hours
		Debt:      165680e6, // milligrams
	}
	// Submit the authorization to the gca server.
	sb := ea.SigningBytes()
	sig := glow.Sign(sb, gcaPriv)
	ea.Signature = sig
	j, err := json.Marshal(ea)
	if err != nil {
		fmt.Println(err)
		return
	}
	/*
		resp, err := http.Post("http://localhost:35015/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(j))
		if err != nil {
			fmt.Println(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			ra, _ := ioutil.ReadAll(resp.Body)
			fmt.Println("NOT OKAY:", string(ra))
			return
		}
	*/
	err = ioutil.WriteFile("auth.json", j, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Save the ShortID for the device.
	var idBytes [4]byte
	binary.LittleEndian.PutUint32(idBytes[:], uint32(sid))
	err = ioutil.WriteFile("short-id.dat", idBytes[:], 0644)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create a history file for the client.
	ts := glow.CurrentTimeslot()
	var tsBytes [4]byte
	binary.LittleEndian.PutUint32(tsBytes[:], ts)
	err = ioutil.WriteFile("history.dat", tsBytes[:], 0644)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = os.MkdirAll("new-client", 0744)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("history.dat", "new-client/history.dat")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("auth.json", "new-client/auth.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("client.keys", "new-client/client.keys")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("gca-servers.dat", "new-client/gca-servers.dat")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("gca.pubkey", "new-client/gca.pubkey")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = CopyFile("short-id.dat", "new-client/short-id.dat")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = sendPendingAuth()
	if err != nil {
		fmt.Println(err)
		return
	}
}
