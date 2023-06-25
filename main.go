package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/opesun/goquery"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

var (
	WORKERS       int             = runtime.NumCPU()      // кол-во "потоков" по числу ядер компьютера
	REPORT_PERIOD int             = 10                    // частота отчетов (сек)
	DUP_TO_STOP   int             = 500                   // максимум повторов до остановки
	HASH_FILE     string          = "hash.bin"            // файл с хэшами
	QUOTES_FILE   string          = "quotes.txt"          // файл с цитатами
	used          map[string]bool = make(map[string]bool) // карта, в качестве ключей используем строки, а значений - бул
)

func init() {
	// Задаем правила разбора
	flag.IntVar(&WORKERS, "w", WORKERS, "количество потоков")
	flag.IntVar(&REPORT_PERIOD, "r", REPORT_PERIOD, "частота отчетов (сек)")
	flag.IntVar(&DUP_TO_STOP, "d", DUP_TO_STOP, "количество дубликатов до остановки")
	flag.StringVar(&HASH_FILE, "hf", HASH_FILE, "файл хэшей")
	flag.StringVar(&QUOTES_FILE, "qf", QUOTES_FILE, "файл записей")
}

// Функция вернет канал, из которого мы будем читать данные типа string
func grab() <-chan string {
	c := make(chan string)
	// В цикле создаем нужное нам количество горутин - worker'ов
	for i := 0; i < WORKERS; i++ {
		go func() {
			for { // В вечном цикле набираем данные
				x, err := goquery.ParseUrl("http://vpustotu.ru/moderation/")
				if err == nil {
					if s := strings.TrimSpace(x.Find(".fi_text").Text()); s != "" {
						c <- s // и отправляем их в канал
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	fmt.Println("Запущено потоков: ", WORKERS)
	return c
}

// функция чтения хэшей из файла, если мы хотим запустить файл программу
// повторно, но не желаем видеть в результирующем файле дубликаты
func readHashes() {
	// проверяем файл на наличие
	if _, err := os.Stat(HASH_FILE); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Файл хэшей не найден, будет создан новый.")
			return
		}
	}

	fmt.Println("Чтение хэшей...")
	hash_file, err := os.OpenFile(HASH_FILE, os.O_RDONLY, 0666)
	check(err)
	defer hash_file.Close()
	// читать будем блоками по 16 байт - как раз один хэш:
	data := make([]byte, 16)
	for {
		n, err := hash_file.Read(data) // n вернет количество прочитанных байт или ошибку
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		if n == 16 {
			used[hex.EncodeToString(data)] = true
		}
	}

	fmt.Println("Завершено. Прочитано хэшей: ", len(used))
}

// функция проверки открытия файлов на ошибки
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	// запускаем разбор аргументов
	flag.Parse()
	// и функцию чтения хэшей
	readHashes()
	// Открываем файл с цитатами...
	quotes_file, err := os.OpenFile(QUOTES_FILE, os.O_APPEND|os.O_CREATE, 0666)
	check(err)
	defer quotes_file.Close()

	// ... и файл с хэшами
	hash_file, err := os.OpenFile(HASH_FILE, os.O_APPEND|os.O_CREATE, 0666)
	check(err)
	defer hash_file.Close()

	// Создаем тикер, который будет оповещать нас, когда пора отчитываться о работе
	ticker := time.NewTicker(time.Duration(REPORT_PERIOD) * time.Second)
	defer ticker.Stop()

	// Создаем канал, который будет ловить сигнал завершения, и привязываем к нему нотификатор
	key_chan := make(chan os.Signal, 1)
	signal.Notify(key_chan, os.Interrupt)

	// ... и все, что нужно для подсчета хэшей
	hasher := md5.New()

	// Счетчик цитат и дубликатов
	quotes_count, dup_count := 0, 0

	quotes_chan := grab()
	for {
		select {
		case quote := <-quotes_chan: // если "пришла" новая цитата:
			quotes_count++
			// считаем хэш и конвертируем его в строку
			hasher.Reset()
			io.WriteString(hasher, quote)
			hash := hasher.Sum(nil)
			hash_string := hex.EncodeToString(hash)
			// проверяем уникальность хэш цитаты
			if !used[hash_string] {
				// все в порядке - заносим хэш в хранилище и записываем его и цитату в файлы
				used[hash_string] = true
				hash_file.Write(hash)
				quotes_file.WriteString(quote + "\n\n\n")
				dup_count = 0
			} else {
				// получен повтор - пришло время проверить, не пора ли закругляться?
				if dup_count++; dup_count == DUP_TO_STOP {
					fmt.Println("Достигнут предел повторов, завершаю работу. Всего записей: ", len(used))
					return
				}
			}
		case <-key_chan: // если пришла информация от нотификатора сигналов:
			fmt.Println("CTRL-C: Завершаю работу. Всего записей: ", len(used))
			return
		case <-ticker.C: // и, наконец, проверяем не пора ли вынести очередной отчет
			fmt.Printf("Всего %d / Повторов %d (%d записей/сек) \n", len(used), dup_count, quotes_count/REPORT_PERIOD)
			quotes_count = 0
		}
	}
}
