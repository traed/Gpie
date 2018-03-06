package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

const name string = "gpie"

var imageDir string

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}

	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}

	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}

	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, url.QueryEscape("drive-go-"+name+".json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)

	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}

	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func setupService() (*drive.Service, error) {
	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved credentials
	// at ~/.credentials/drive-go-XXX.json
	config, err := google.ConfigFromJSON(b, drive.DriveReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	client := getClient(ctx, config)

	return drive.New(client)
}

func getFilesList() map[string]string {
	files, err := ioutil.ReadDir(imageDir)
	if err != nil {
		log.Fatal(err)
	}

	names := make(map[string]string)
	for _, f := range files {
		s := f.Name()
		i := strings.Index(s, ".")
		names[s[:i]] = s[i:]
	}

	return names
}

func getFilesListString() string {
	files := getFilesList()

	var str string
	for name, ext := range files {
		str += imageDir + "/" + name + ext + " "
	}

	return str
}

func downloadFile(srv *drive.Service, id string, filepath string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := srv.Files.Get(id).Download()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getExtension(name string) string {
	i := strings.Index(name, ".")
	return name[i:len(name)]
}

func updateFiles() {
	srv, err := setupService()
	if err != nil {
		log.Fatalf("Unable to setup drive Client %v", err)
	}

	res, err := srv.Files.List().Fields("nextPageToken, files(id, name, md5Checksum)").Q("'1p8uFsCYf90m4IseWBHV3CFTVXKxszyuX' in parents").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}

	files := getFilesList()

	if len(res.Files) > 0 {
		for _, i := range res.Files {
			if _, exists := files[i.Md5Checksum]; exists {
				err := downloadFile(srv, i.Id, fmt.Sprintf("%s/%s%s", imageDir, i.Md5Checksum, getExtension(i.Name)))
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func runFbi() {
	files := getFilesListString()
	output, err := exec.Command("fbi", "-a", "-noverbose", "-norandom", "-t 8", files).CombinedOutput()
	if err != nil {
		log.Fatalf("Unable to start fbi: %v", string(output))
	}
}

func main() {
	imageDir, _ = filepath.Abs("images")
	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		err := os.MkdirAll(imageDir, 0775)
		if err != nil {
			log.Fatalf("Unable to create image dir: %v", err)
		}
	}

	updateFiles()
	runFbi()

	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ticker.C:
			exec.Command("pkill fbi").Run()
			updateFiles()
			runFbi()
		}
	}
}
