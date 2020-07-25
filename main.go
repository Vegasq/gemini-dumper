package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/makeworld-the-better-one/go-gemini"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

type SavePage struct {
	url   string
	mime  string
	cache string
}

func getExtFromUrl(u string) string {
	// Check if its conventional URL, not all URL in gemini will fit it tho
	url, err := url.Parse(u)
	if err != nil {
		return "noext"
	}

	dotSplitted := strings.Split(url.Path, ".")

	// For case when it's just domain, and no actual path
	if len(dotSplitted) == 1 {
		return "noext"
	}

	// Ensure ext fits regex a-zA-Z0-9
	re := regexp.MustCompile(`^([a-zA-Z0-9]{1,5})$`)
	if re.MatchString(dotSplitted[len(dotSplitted)-1]) {
		return dotSplitted[len(dotSplitted)-1]
	}

	return "noext"
}

// getUrlHash returns md5hash of a page with some extention
func getUrlHash(pageUrl string) string {
	hash := md5.Sum([]byte(pageUrl))
	return fmt.Sprintf("%x.%s", hash, getExtFromUrl(pageUrl))
}

func getUrlCacheLocation(url string) (string, bool) {
	hashValue := getUrlHash(url)
	fileLocation := path.Join("db", hashValue)

	if fileExists(fileLocation) {
		return fileLocation, true
	}
	return fileLocation, false
}

func createFile(location string) *os.File {
	for {
		f, err := os.Create(location)
		if err != nil {
			fmt.Println("os.Create", err, location)
			time.Sleep(time.Second * 1)
			continue
		}
		return f
	}
}

func savePage(save chan SavePage, url string, body *bytes.Buffer, mime string) {
	location, exists := getUrlCacheLocation(url)
	if exists {
		return
	}

	f := createFile(location)
	defer f.Close()
	_, err := f.Write(body.Bytes())
	if err != nil {
		fmt.Println("f.Write:", err)
	}
	err = f.Close()
	if err != nil {
		fmt.Println("f.Close:", err)
	}

	save <- SavePage{url, mime, getUrlHash(url)}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func downloadPage(url string) (*bytes.Buffer, string) {
	var client = &gemini.Client{Timeout: 5 * time.Second}
	client.Insecure = true
	var res gemini.Response

	i := 0
	for {
		i = i + 1
		if i == 1000 {
			fmt.Printf("Failed to retrieve %s\n", url)
			return &bytes.Buffer{}, ""
		}

		response, err := client.Fetch(url)

		if err != nil {
			time.Sleep(120 * time.Second)
			continue
		}
		res = *response
		break
	}

	bts := new(bytes.Buffer)
	_, err := io.Copy(bts, res.Body)
	if err != nil {
		fmt.Printf("io copy error %s\n", err)
	}

	return bts, res.Meta
}

type PageInfo struct {
	name string
	url  string
}


func populateURL(originalLink, actualLink string) (string, error) {
	if strings.Index(actualLink, "://") != -1 {
		// Full link
		if strings.Index(actualLink, "gemini://") == -1 {
			// Skip non gemini sites
			return "", errors.New("Non gemini link")
		}
	} else {
		// Partial link
		u, err := url.Parse(originalLink)

		if err != nil {
			fmt.Println("Failed to parse", originalLink)
			return "", errors.New(fmt.Sprintf("Failed to parse %s\n", originalLink))
		}

		if len(actualLink) != 0 && string([]rune(actualLink)[0]) == "/" {
			u.Path = actualLink
		} else if len(actualLink) != 0 {
			u.Path = path.Join(getPathToCurrentDir(u.Path), actualLink)
		} else {
			return "", errors.New(fmt.Sprintf("Failed to parse %s\n", originalLink))
		}

		actualLink = u.String()
	}
	return actualLink, nil
}


func parseGmiUrl(gmiURL string) (string, string) {
	linkName := ""
	noTag := strings.Replace(gmiURL, "=> ", "", 1)
	splitByTab := strings.Split(noTag, "\t")
	if len(splitByTab) == 2 {
		linkName = splitByTab[1]
	}
	splitBySpace := strings.Split(splitByTab[0], " ")
	if len(linkName) == 0 && len(splitByTab) == 2 {
		linkName = splitByTab[1]
	}
	return splitBySpace[0], linkName
}


func extractLinks(linkToCheck, body string) []PageInfo {
	linkSearch, err := regexp.Compile(`(?mi)^=> (.*)$`)
	if err != nil {
		fmt.Printf("Failed to compile regex %s\n", err)
	}
	foundLinks := linkSearch.FindAllString(body, 500)
	var linksToCheck []PageInfo

	for i := range foundLinks {
		actualLink, linkName := parseGmiUrl(foundLinks[i])
		actualLink, err = populateURL(linkToCheck, actualLink)
		if err != nil {
			fmt.Printf("Failed to parse link: %s", err)
			continue
		}
		linksToCheck = append(linksToCheck, PageInfo{name: linkName, url: actualLink})
	}
	return linksToCheck
}

func urlHandler(urls chan string, save chan SavePage) {
	url := <-urls
	fmt.Println("Handle url", url)
	body, mime := downloadPage(url)

	savePage(save, url, body, mime)

	newLinks := extractLinks(url, string(body.Bytes()))
	for ni := range newLinks {
		_, exists := getUrlCacheLocation(newLinks[ni].url)
		if exists == false {
			fmt.Println("Link to check", newLinks[ni].url, newLinks[ni].name)
			urls <- newLinks[ni].url
		}
	}
}


func getPathToCurrentDir(path string) string {
	if len(path) == 0 {
		return path
	}
	if path[len(path)-1] == '/' {
		return path
	}
	pieces := strings.Split(path, "/")
	return strings.Join(pieces[0:len(pieces)-1], "/") + "/"
}


type DB struct {
	fl os.File
	ch *chan SavePage
}

func (db *DB) Save() {
	fl, err := os.OpenFile("save.db", os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModeAppend)
	if err != nil {
		panic("Can not open DB")
	}

	db.fl = *fl

	for {
		page := <-*db.ch
		_, err := fl.WriteString(fmt.Sprintf("%s,%s,%s\n", page.url, page.cache, page.mime))
		if err != nil {
			panic(fmt.Sprintf("Failed to save %s with %s\n", page, err))
		}
	}
}

func (db *DB) Close() {
	db.fl.Close()
}

func main() {
	ch := make(chan string, 100000)
	ch <- "gemini://home.mkla.dev/"
	ch <- "gemini://gemini.circumlunar.space/"

	saveChan := make(chan SavePage)

	db := DB{ch: &saveChan}
	go db.Save()

	for {
		if len(ch) > 0 {
			go urlHandler(ch, saveChan)
			fmt.Printf("%d pages to check\n", len(ch))
		}
	}
}
