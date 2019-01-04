package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cloud.google.com/go/translate"
	"golang.org/x/text/language"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getSubString(filename string) string {
	cmd := exec.Command("ffmpeg", "-txt_format", "text", "-i", filename, "-f", "srt", "-")
	output := new(bytes.Buffer)
	cmd.Stdout = output
	cmd.Run()
	return output.String()
}

func writeSubs(filename, outfile, subtitles string) {
	subFormat := "ass"
	if strings.HasSuffix(outfile, ".mp4") {
		subFormat = "mov_text"
	}

	cmd := exec.Command("ffmpeg", "-i", filename, "-i", "-", "-map", "0:v:0", "-map", "0:a:0", "-map", "1", "-c", "copy", "-c:s", subFormat, outfile)
	printTo, err := cmd.StdinPipe()
	cmd.Start()
	if err != nil {
		panic(err)
	}

	printTo.Write([]byte(subtitles))
	printTo.Close()

	cmd.Wait()
}

type caption struct {
	num    string
	timing string
	text   string
}

type captionbunch struct {
	captions []*caption
}

func (c *captionbunch) addCaption(caption *caption) {
	c.captions = append(c.captions, caption)
}

func (c *captionbunch) translate(start, end int) {
	texts := make([]string, end-start)
	for i := start; i < end; i++ {
		texts[i-start] = c.captions[i].text
	}

	ctx := context.Background()
	client, err := translate.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	translations, err := client.Translate(ctx,
		texts, language.Bulgarian, &translate.Options{
			Source: language.English,
			Format: translate.HTML,
		})
	if err != nil {
		panic(err)
	}

	for idx, translation := range translations {
		c.captions[idx+start].text = translation.Text + "\n\n"
	}
}

func (c *captionbunch) translateAll() {
	prevPos := 0
	step := 10
	for prevPos < len(c.captions) {
		pos := min(prevPos+step, len(c.captions))
		c.translate(prevPos, pos)
		prevPos = pos
	}
}

func (c *captionbunch) toString() string {
	sb := strings.Builder{}
	for _, c := range c.captions {
		sb.WriteString(c.num)
		sb.WriteString(c.timing)
		sb.WriteString(c.text)
	}
	return sb.String()
}

func main() {

	allSubs := getSubString(os.Args[1])
	subParts := strings.Split(allSubs, "\n\n")

	cp := captionbunch{make([]*caption, 0)}

	for _, subPart := range subParts {
		if len(subPart) < 5 {
			continue
		}
		reader := bufio.NewReader(strings.NewReader(subPart))

		// subtitle number
		subtitleNumber, _ := reader.ReadBytes('\n')
		// timing
		subtitleTiming, _ := reader.ReadBytes('\n')
		// text
		subtitleText, _ := reader.ReadString(byte(0))

		cp.addCaption(&caption{string(subtitleNumber), string(subtitleTiming), string(subtitleText) + "\n\n"})
	}

	fmt.Println("Extracted subs... Translating")

	cp.translateAll()

	fmt.Println("Translated subs... Writing new vide file")

	translatedSubs := cp.toString()
	//fmt.Println(translatedSubs)
	writeSubs(os.Args[1], os.Args[2], translatedSubs)
}
