// Get typing test words from different sources

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
)

func randomText(words []string, numwords int) string {
	// Return random list of numwords from words
	var returnWords []string
	for i := 0; i < numwords; i++ {
		// Append random word to returnWords
		random_index := rand.Intn(len(words))
		returnWords = append(returnWords, words[random_index])
	}

	// Return the words as a joined string
	return strings.Join(returnWords, " ")
}

// 从文件中生成单词
// 格式为
//
//	{
//	 "name": "english_1k",
//	  "noLazyMode": true,
//	  "orderedByFrequency": true,
//	  "words": [
//	    "the",
//	    "be",
//	    ...
//	 ]
//	}

type wordTestFile struct {
	Name               string   `json:"name"`
	NoLazyMode         bool     `json:"noLazyMode"`
	OrderedByFrequency bool     `json:"orderedByFrequency"`
	Words              []string `json:"words"`
}

func generateWordsTestFromFile(filename string, numwords int, numsegments int) func() []segment {
	// fmt.Println("Reading from file: " + filename)
	res, err := os.ReadFile(fmt.Sprintf("./data/words/%s.json", filename))
	if err != nil {
		exit("%s does not appear to be a valid word file, use -list words to see a list of supported word lists", filename)
	}

	// Parse the file and get the words
	name := strings.Split(filename, ".")[0]

	// words := make([]string, 0)
	var wordTestFile wordTestFile
	var words []string
	err = json.Unmarshal(res, &wordTestFile)
	if err != nil {
		exit("Error parsing word file: %s", err)
	}

	words = wordTestFile.Words

	// Return a function that
	return func() []segment {
		segments := make([]segment, numsegments)
		for i := 0; i < numsegments; i++ {
			segments[i] = segment{Text: randomText(words, numwords), Attribution: name}
		}
		return segments
	}
}

type quoteTestFile struct {
	Language string  `json:"language"`
	Groups   [][]int `json:"groups"`
	Quotes   []struct {
		Text   string `json:"text"`
		Source string `json:"source"`
		Length int    `json:"length"`
		ID     int    `json:"id"`
	} `json:"quotes"`
}

// 从quotes/选一个
// 格式为
// {
// "language": "english",
//
//	  "groups": [
//	    [0, 100],
//	    [101, 300],
//	    [301, 600],
//	    [601, 9999]
//	  ],
//	  "quotes": [
//	    {
//	      "text": "You have the power to heal your life, and you need to know that.",
//	      "source": "Meditations to Heal Your Life",
//	      "length": 64,
//	      "id": 1
//	    },
//	    ...
//	  ]
//	}
func generateQuoteTestFromFile(filename string) func() []segment {
	res, err := os.ReadFile(fmt.Sprintf("./data/quotes/%s.json", filename))
	if err != nil {
		exit("%s does not appear to be a valid quote file, use -list quotes to see a list of supported quote lists", filename)
	}

	// Parse the file and get the quotes
	var quoteTestFile quoteTestFile
	err = json.Unmarshal(res, &quoteTestFile)
	if err != nil {
		exit("Error parsing quote file: %s", err)
	}

	quotes := quoteTestFile.Quotes

	// Return a function that gets a random quote
	return func() []segment {
		var randomIndex = rand.Intn(len(quotes))
		return []segment{{Text: quotes[randomIndex].Text, Attribution: quotes[randomIndex].Source}}
	}

}

// TODO 用LLM来生成单词
func generateTestFromLLM(testtype string, llm string, numwords int) func() []segment {
	// Use go to send a request to the ollama server and get words
	fmt.Println("Generating words from LLM", testtype)
	prompt := ""

	if testtype == "words" {
		prompt = "Generate random " + fmt.Sprint(numwords) + " words separated by spaces. Only include words that are in the English language. The words should be common and easy to type. The words should be in lowercase. No additional text."
	} else if testtype == "quote" {
		prompt = "Generate a famous quote in English. No additional text but the quote itself. Don't include the quotation marks."
	}

	// prompt = "Output " + fmt.Sprint(numwords) + " common, easy-to-type English words in lowercase, separated by spaces, with no additional text."

	// Send a curl request to the ollama server
	// curl http://localhost:11434/api/generate -d '{
	//   "model": "llama2",
	//   "prompt": "Why is the sky blue?",
	//   "stream": false
	// }'

	// Send curl request
	cmd := exec.Command("curl", "http://localhost:11434/api/generate", "-d", fmt.Sprintf(`{"model": "%s", "prompt": "%s", "stream": false}`, llm, prompt))

	// Get the output
	output, err := cmd.Output()
	if err != nil {
		exit("Error getting words from LLM: %s", err)
	}

	// Output located in a string of {"response": "words"}
	// Parse the output
	type LLMResponse struct {
		Model                string  `json:"model"`
		Response             string  `json:"response"`
		Created_at           string  `json:"created_at"`
		Done                 bool    `json:"done"`
		Context              []int64 `json:"context"`
		Total_duration       int64   `json:"total_duration"`
		Load_duration        int64   `json:"load_duration"`
		Prompt_eval_count    int64   `json:"prompt_eval_count"`
		Prompt_eval_duration int64   `json:"prompt_eval_duration"`
		Eval_count           int64   `json:"eval_count"`
		Eval_duration        int64   `json:"eval_duration"`
	}

	var llmResponse LLMResponse
	err = json.Unmarshal(output, &llmResponse)
	if err != nil {
		exit("Error parsing LLM response: %s", err)
	}

	final_output := llmResponse.Response

	// Return a function that gets the words
	return func() []segment {
		return []segment{{Text: final_output, Attribution: llm}}
	}
}
