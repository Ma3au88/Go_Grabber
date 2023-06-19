package main

import (
	"flag"
	"fmt"
	"github.com/opesun/goquery"
	"strings"
	"time"
)

var (
	WORKERS       int    = 2            // кол-во "потоков"
	REPORT_PERIOD int    = 10           // частота отчетов (сек)
	DUP_TO_STOP   int    = 500          // максимум повторов до остановки
	HASH_FILE     string = "hash.bin"   // файл с хэшами
	QUOTES_FILE   string = "quotes.txt" //файл с цитатами
)

func init() {
	// Задаем правила разбора
	flag.IntVar(&WORKERS, "w", WORKERS, "количество потоков")
	flag.IntVar(&REPORT_PERIOD, "r", REPORT_PERIOD, "частота отчетов (сек)")
	flag.IntVar(&DUP_TO_STOP, "d", DUP_TO_STOP, "количество дубликатов до остановки")
	flag.StringVar(&HASH_FILE, "hf", HASH_FILE, "файл хэшей")
	flag.StringVar(&QUOTES_FILE, "qf", QUOTES_FILE, "файл записей")

	// И запускаем разбор аргументов
	flag.Parse()
}

// Функция вернет канал, из которого мы будем читать данные типа string
func grab() <-chan string {
	c := make(chan string)
	// В цикле создаем нужное нам количество горутин - worker'ов
	for i := 0; i < WORKERS; i++ {
		go func() {
			for { // В вченом цикле набираем данные
				x, err := goquery.ParseUrl("http://vpustotu.ru/moderation/")
				if err == nil {
					if s := strings.TrimSpace(x.Find(".fi_text").Text()); s != "" {
						c <- s // и отправлем их в канал
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	fmt.Println("Запущено потоков: ", WORKERS)
	return c
}

func main() {
	quote_chan := grab()

	for i := 0; i < 5; i++ { // Получаем 5 цитат
		fmt.Println(<-quote_chan, "\n")
	}
}
