package mercuriy

import (
	"bytes"
	"cto_ksm_mercury/consttypes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

var testnomsessii int

var resMerc consttypes.TAnswerMercur

func GetSNOByDefault(emulation bool, ipktt string, port int, sessionkey string) (int, error) {
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"GetRegistrationInfo\"}", sessionkey))
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду получения регистрационной информации")
	}
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		return -1, err
	}
	err = json.Unmarshal(buffAnsw, &resMerc)
	if err != nil {
		return -1, err
	}
	if resMerc.Result != 0 {
		err = fmt.Errorf(resMerc.Description)
		if !emulation {
			return -1, err
		} else {
			resMerc.RegistrationInfo = new(consttypes.TMercRegistrationInfo)
			resMerc.RegistrationInfo.TaxSystem = []int{5}
		}
	}
	if len(resMerc.RegistrationInfo.TaxSystem) != 1 {
		err := errors.New("касса зарегистрирована на больше чем одна система налогообложение")
		fmt.Println(err)
		return -1, err
	}
	return resMerc.RegistrationInfo.TaxSystem[0], nil
}

func OpenCloseShift(emulation bool, ipktt string, port int, comPort int, userint int, passwuser string, numReport int, sessionkey string, open bool, cashier string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	var errorOfPrintReport error
	textDescription := "открытие"
	if !open {
		textDescription = "закрытие"
	}
	if sessionkey == "" {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Отправляем команду %v сессии", textDescription)
		}
		answer, err := opensession(ipktt, port, comPort, userint, passwuser)
		if err != nil {
			descrError := fmt.Sprintf("ошибка %v сессии к ккт меркурий", textDescription)
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Ответ: %s", answer)
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := fmt.Sprintf("ошибка при разобре ответа при %v сессии покдлючения к ККТ меркурий", textDescription)
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := fmt.Sprintf("ошибка при %v сессии к ккт меркурий", textDescription)
			err = fmt.Errorf(resMerc.Description)
			err = errors.Join(err, errors.New(descrError))
			if !emulation {
				return descrError, err
			} else {
				testnomsessii = testnomsessii + 1
				resMerc.SessionKey = "эмуляция" + strconv.Itoa(testnomsessii)
			}
		}
		sessionkey = resMerc.SessionKey
		defer func() {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("Закрываем сессию с ключом: %v", sessionkey)
			}
			Closesession(ipktt, port, &sessionkey)
		}()
	}
	commandShift := "OpenShift"
	if !open {
		commandShift = "CloseShift"
	}
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"%v\", \"cashierInfo\": { \"cashier\": \"%v\" }}", sessionkey, commandShift, cashier))
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду %v смены: %v", commandShift, string(jsonmerc))
	}
	buffAnsw, errorOfPrintReport := sendCommandTCPMerc(jsonmerc, ipktt, port)
	return string(buffAnsw), errorOfPrintReport
}

func PrintReport(emulation bool, ipktt string, port int, comPort int, userint int, passwuser string, numReport int, sessionkey string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	var errorOfPrintReport error
	if sessionkey == "" {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Отправляем команду открытия сессии")
		}
		answer, err := opensession(ipktt, port, comPort, userint, passwuser)
		if err != nil {
			descrError := "ошибка открытия сессии к ккт меркурий"
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Ответ: %s", answer)
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := "ошибка при разобре ответа при отрытии сессии покдлючения к ККТ меркурий"
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := "ошибка при подключении к ккт меркурий"
			err = fmt.Errorf(resMerc.Description)
			err = errors.Join(err, errors.New(descrError))
			if !emulation {
				return descrError, err
			} else {
				testnomsessii = testnomsessii + 1
				resMerc.SessionKey = "эмуляция" + strconv.Itoa(testnomsessii)
			}
		}
		sessionkey = resMerc.SessionKey
		defer func() {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("Закрываем сессию с ключом: %v", sessionkey)
			}
			Closesession(ipktt, port, &sessionkey)
		}()
	}
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"PrintReport\", \"PrintReport\":%v}", sessionkey, numReport))
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду печати отчета: %v", string(jsonmerc))
	}
	buffAnsw, errorOfPrintReport := sendCommandTCPMerc(jsonmerc, ipktt, port)
	return string(buffAnsw), errorOfPrintReport
}

func RunProcessCheckMark(emulation bool, ipktt string, port int, countAttemptsOfMarkCheck int, pauseOfMarksMistake int, sessionkey string, mark string) (consttypes.TItemInfoCheckResult, error) {
	var countAttempts int
	//var imcResultCheckinObj consttypes.TItemInfoCheckResultObject
	var imcResultCheckin consttypes.TItemInfoCheckResult
	//проверяем - открыта ли смена
	//shiftOpenned, err := checkOpenShift(true, "админ")
	//if err != nil {
	//	errorDescr := fmt.Sprintf("ошибка (%v). Смена не открыта", err)
	//	return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	//}
	//if !shiftOpenned {
	//	errorDescr := fmt.Sprintf("ошибка (%v) - смена не открыта", err)
	//	return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	//}
	//посылаем запрос на проверку марки
	var resJson string
	var err error
	var resMercAnswerBytes []byte
	var answerMerc consttypes.TAnswerMercur
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду проверки марки %v", mark)
	}
	resMercAnswerBytes, err = SendCheckOfMark(ipktt, port, sessionkey, mark, 0)
	if err == nil {
		err = json.Unmarshal(resMercAnswerBytes, &answerMerc)
		if err == nil {
			resJson = answerMerc.Description
			if answerMerc.Result != 0 && !emulation {
				resJson = "error " + resJson
			}
		}
	}
	if err != nil {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка (%v) запуска проверки марки %v", err, mark)
		}
		errorDescr := fmt.Sprintf("ошибка (%v) запуска проверки марки %v", err, mark)
		return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	}
	if !successCommand(resJson) {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка (%v) запуска проверки марки %v", resJson, mark)
		}
		errorDescr := fmt.Sprintf("ошибка (%v) запуска проверки марки %v", resJson, mark)
		return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	}
	for countAttempts = 0; countAttempts < countAttemptsOfMarkCheck; countAttempts++ {
		var answerOfCheckMark consttypes.TAnsweChekcMark
		var MercurAnswerOfCheckMark consttypes.TAnswerMercur
		var resMercAnswerBytes []byte
		resMercAnswerBytes, err = GetStatusOfChecking(ipktt, port, sessionkey)
		if err == nil {
			err = json.Unmarshal(resMercAnswerBytes, &MercurAnswerOfCheckMark)
			if err == nil {
				resJson = MercurAnswerOfCheckMark.Description
				if MercurAnswerOfCheckMark.Result != 0 && !emulation {
					resJson = "error " + resJson
				}
				if MercurAnswerOfCheckMark.Result != 0 && emulation {
					MercurAnswerOfCheckMark.IsCompleted = true
				}
			}
		}
		if err != nil {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("ошибка (%v) получения статуса проверки марки %v", err, mark)
			}
			errorDescr := fmt.Sprintf("ошибка (%v) получения статуса проверки марки %v", err, mark)
			return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
		}
		if !successCommand(resJson) {
			//делаем паузу
			desrAction := fmt.Sprintf("пауза в %v секунд... так сервер провекри марок не успевает.", pauseOfMarksMistake)
			if consttypes.Logger != nil {
				consttypes.Logger.Println(desrAction)
			}
			duration := time.Second * time.Duration(pauseOfMarksMistake)
			time.Sleep(duration)
			//if strings.Contains(resJson, "421")
			//errorDescr := fmt.Sprintf("ошибка (%v) получения статуса проверки марки %v", resJson, mark)
			//logsmy.Logsmap[consttypes.LOGERROR].Println(errorDescr)
			//return TItemInfoCheckResult{}, errors.New(errorDescr)
		}
		if answerOfCheckMark.Ready {
			if emulation && (countAttempts < countAttemptsOfMarkCheck-20) {
				//емулируем задержку полчение марки
			} else {
				break
			}
		}
		//пауза в 1 секунду
		duration := time.Second
		time.Sleep(duration)
	}
	if countAttempts == countAttemptsOfMarkCheck {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка проверки марки %v", mark)
		}
		errorDescr := fmt.Sprintf("ошибка проверки марки %v", mark)
		return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	}
	//принимаем марку
	var resOfChecking string
	var MercurAnswerOfResultOfCheckMark consttypes.TAnswerMercur
	resMercAnswerBytes, err = AcceptMark(ipktt, port, sessionkey)
	if err == nil {
		err = json.Unmarshal(resMercAnswerBytes, &MercurAnswerOfResultOfCheckMark)
		if err == nil {
			resOfChecking = MercurAnswerOfResultOfCheckMark.Description
			if MercurAnswerOfResultOfCheckMark.Result != 0 && !emulation {
				resOfChecking = "error " + resJson
			}
		}
	}
	if err != nil {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка (%v) принятия марки %v", err, mark)
		}
		errorDescr := fmt.Sprintf("ошибка (%v) принятия марки %v", err, mark)
		return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	}
	if !successCommand(resOfChecking) {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка (%v) принятия марки %v", resOfChecking, mark)
		}
		errorDescr := fmt.Sprintf("ошибка (%v) принятия марки %v", resOfChecking, mark)
		return consttypes.TItemInfoCheckResult{}, errors.New(errorDescr)
	}
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("конец процедуры runProcessCheckMark без ошибки")
	}
	return imcResultCheckin, nil
} //runProcessCheckMark

func PrintCheck(emulation bool, ipktt string, port int, comport int, checkdoc consttypes.TDocument, sessionkey string, snoDefault int, dontprintrealfortest bool, userint int, passwuser string, emulatmistakesOpenCheck bool) (string, error) {
	var resMerc, resMercCancel consttypes.TAnswerMercur
	var answer []byte
	var answerclosecheck []byte
	var errclosecheck, errOfOpenCheck error
	if sessionkey == "" {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Отправляем команду открытия сессии")
		}
		answer, err := opensession(ipktt, port, comport, userint, passwuser)
		if err != nil {
			descrError := "ошибка открытия сессии к ккт меркурий"
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := "ошибка при разобре ответа при отрытии сессии покдлючения к ККТ меркурий"
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := "ошибка при подключении к ккт меркурий"
			err = fmt.Errorf(resMerc.Description)
			err = errors.Join(err, errors.New(descrError))
			if !emulation {
				return descrError, err
			} else {
				testnomsessii = testnomsessii + 1
				resMerc.SessionKey = "эмуляция" + strconv.Itoa(testnomsessii)
			}
		}
		sessionkey = resMerc.SessionKey
		defer func() {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("Закрываем сессию с ключом: %v", sessionkey)
			}
			Closesession(ipktt, port, &sessionkey)
		}()
	}

	if snoDefault == -1 {
		var err error
		snoDefault, err = GetSNOByDefault(emulation, ipktt, port, sessionkey)
		if err != nil {
			descrError := "ошибка получения системы налогообложения смены по умолчанию"
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
	}

	checheaderkmerc, err := convertDocToMercHeader(checkdoc, snoDefault)
	checheaderkmerc.SessionKey = sessionkey
	if err != nil {
		descrError := "ошибка конвертации шапки чека атол в шапку чека меркурия"
		err = errors.Join(err, errors.New(descrError))
		return descrError, err
	}
	headercheckmerc, err := json.Marshal(checheaderkmerc)
	if err != nil {
		descrError := fmt.Sprintf("ошибка формирования шапки чека для кассы меркурий из структуры (%v)", checheaderkmerc)
		err = errors.Join(err, errors.New(descrError))
		return descrError, err
	}
	answer, err = opencheck(ipktt, port, headercheckmerc)
	if err != nil {
		descrError := "ошибка открытия чека для кассы меркурий"
		err = errors.Join(err, errors.New(descrError))
		return descrError, err
	}
	err = json.Unmarshal(answer, &resMerc)
	if err != nil {
		descrError := "ошибка разбора ответа при открытии чека для кассы меркурий"
		err = errors.Join(err, errors.New(descrError))
		return descrError, err
	}
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("resMerc.Result1: %v", resMerc.Result)
	}
	if resMerc.Result != 0 { //если не получилось открыть чек, отменяем его и пробуем отрыть заново
		descrError := fmt.Sprintf("ошибка (%v) открытия чека для кассы меркурий (попытка 1)", resMerc.Description)
		errOfOpenCheck = errors.New(descrError)
		answerCancel, errCancel := cancelcheck(ipktt, port, &sessionkey) //отменяем предыдущий чек
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Ответ на команду cancelcheck: %v", string(answerCancel))
		}
		if errCancel != nil {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("ошибка (%v) отмены1 чека для кассы меркурий", errCancel)
			}
			errOfOpenCheck = errors.Join(errOfOpenCheck, errCancel)
		} else {
			errUnMarshCancel := json.Unmarshal(answerCancel, &resMercCancel)
			if errUnMarshCancel != nil {
				if consttypes.Logger != nil {
					consttypes.Logger.Printf("ошибка (%v) разбора ответа отмены2 чека для кассы меркурий", errUnMarshCancel)
				}
				errOfOpenCheck = errors.Join(errOfOpenCheck, errUnMarshCancel)
			} else {
				if resMercCancel.Result != 0 {
					if consttypes.Logger != nil {
						consttypes.Logger.Printf("ошибка (%v) отмены3 чека для кассы меркурий", resMercCancel.Description)
					}
					descrError := fmt.Sprintf("ошибка (%v) отмены чека для кассы меркурий", resMercCancel.Description)
					errOfOpenCheck = errors.Join(errOfOpenCheck, errors.New(descrError))
				} else {
					if consttypes.Logger != nil {
						consttypes.Logger.Printf("Открываем чек заново")
					}
					answer, err = opencheck(ipktt, port, headercheckmerc) //открываем заново чек
					if consttypes.Logger != nil {
						consttypes.Logger.Printf("Ответ на команду opencheck2: %v", string(answer))
					}
					if err == nil {
						err = json.Unmarshal(answer, &resMerc) //разбираем ответ
						if consttypes.Logger != nil {
							consttypes.Logger.Printf("результат открытия чека: %v", resMerc.Result)
						}
						if err != nil {
							fmt.Printf("ошибка (%v) разбора ответа отмены чека\n", err)
						}
					}
				}
			}
		}
	}
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Ответ на команду opencheck3: %v", string(answer))
	}
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("resMerc.Result2: %v", resMerc.Result)
	}
	if resMerc.Result != 0 { //если не получилось открыть чек
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("ошибка (%v) открытия чека для кассы меркурий", resMerc.Description)
		}
		descrError := fmt.Sprintf("ошибка (%v) открытия чека для кассы меркурий", resMerc.Description)
		err = errors.Join(errOfOpenCheck, errors.New(descrError))
		if !emulation {
			return descrError, err
		}
	}

	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Всего позиций: %v", len(checkdoc.Items))
	}

	//проверка марок
	for _, pos := range checkdoc.Items {
		if pos.Mark == "" {
			continue
		}
		_, err := RunProcessCheckMark(emulation, ipktt, port, 10, 10, sessionkey, pos.Mark)
		if err != nil {
			descrError := fmt.Sprintf("ошибка (%v) проверки марки %v", err, pos.Mark)
			err = errors.Join(err, errors.New(descrError))
			if consttypes.Logger != nil {
				consttypes.Logger.Println(descrError)
			}
			return descrError, err
		}
	}

	for _, pos := range checkdoc.Items {
		//var currPos consttypes.TPosition
		//mapstructure.Decode(pos, &currPos)
		mercPos, err := convertDocPosToMercPos(pos, checkdoc.IsReturn)
		mercPos.SessionKey = sessionkey
		if err != nil {
			descrError := fmt.Sprintf("ошибка формирования структуры позиции для кассы меркурий из позиции json-задания (%v)", pos)
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		mercPosJsonBytes, err := json.Marshal(mercPos)
		if err != nil {
			descrError := fmt.Sprintf("ошибка маршалинга структуры позиции для кассы меркурий из (%v)", mercPos)
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		answer, err = addpos(ipktt, port, mercPosJsonBytes)
		if err != nil {
			descrError := fmt.Sprintf("ошибка добавления позиции %v в чек для кассы меркурий", mercPosJsonBytes)
			err = errors.Join(err, errors.New(descrError))
			if !emulation {
				return descrError, err
			}
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := fmt.Sprintf("ошибка маршалинга результата %v добавления позиции в чек для кассы меркурий", resMerc)
			err = errors.Join(err, errors.New(descrError))
			return descrError, err
		}
		if resMerc.Result != 0 {
			descrError := fmt.Sprintf("ошибка добавления позиции %v в чек для кассы меркурий", mercPosJsonBytes)
			err = fmt.Errorf(resMerc.Description)
			err = errors.Join(err, errors.New(descrError))
			if !emulation {
				return descrError, err
			}
		}
	}
	checkclosekmerc := convertDocMercCloseCheck(checkdoc)
	checkclosekmerc.SessionKey = sessionkey
	checkclosekmercbytes, err := json.Marshal(checkclosekmerc)
	if err != nil {
		descrError := "ошибка формирования данных для закрытия чек кассы меркурий"
		err = errors.Join(err, errors.New(descrError))
		return descrError, err
	}
	if !dontprintrealfortest {
		answerclosecheck, errclosecheck = closecheck(ipktt, port, checkclosekmercbytes)
	} else {
		answerclosecheck, errclosecheck = cancelcheck(ipktt, port, &sessionkey)
	}
	if errclosecheck != nil {
		descrError := "ошибка закрытия чека для кассы меркурий"
		errclosecheck = errors.Join(errclosecheck, errors.New(descrError))
		return descrError, errclosecheck
	}
	errclosecheck = json.Unmarshal(answerclosecheck, &resMerc)
	if errclosecheck != nil {
		descrError := "ошибка разбора резульата закрытия чека для кассы меркурий"
		errclosecheck = errors.Join(errclosecheck, errors.New(descrError))
		return descrError, errclosecheck
	}
	if resMerc.Result != 0 {
		descrError := "ошибка закрытия чека для кассы меркурий"
		errclosecheck = errors.New(resMerc.Description)
		errclosecheck = errors.Join(errclosecheck, errors.New(descrError))
		if !emulation {
			return descrError, errclosecheck
		} else {
			errclosecheck = nil
		}
	}
	return string(answerclosecheck), errclosecheck
} //PrintCheck

func CheckStatsuConnectionKKT(emulation bool, ipktt string, port int, comport int, sessionkey string, userint int, passwuser string) (string, string, error) {
	var resMerc consttypes.TAnswerMercur
	answerbytesserver, errStatusServer := getStatusServerKKT(ipktt, port)
	if errStatusServer != nil {
		descrError := "ошибка получения статуса сервера ккт меркурий"
		return "", descrError, errStatusServer
	}
	errUnmarshServer := json.Unmarshal(answerbytesserver, &resMerc)
	if errUnmarshServer != nil {
		descrError := fmt.Sprintf("ошибка распаковки ответа %v сервера ккт меркурий", string(answerbytesserver))
		return "", descrError, errUnmarshServer
	}
	if resMerc.Result != 0 {
		descrError := fmt.Sprintf("сервер ККТ меркурий не работает по причине %v", resMerc.Description)
		err := errors.New(resMerc.Description)
		return "", descrError, err
	}
	if sessionkey == "" {
		answer, err := opensession(ipktt, port, comport, userint, passwuser)
		if err != nil {
			descrError := "ошибка при подключении к ккт меркурий"
			return "", descrError, err
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := fmt.Sprintf("ошибка при разборе ответа %v от ккт меркурий", answer)
			return "", descrError, err
		}

		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := "ошибка при подключении к ккт меркурий"
			err = fmt.Errorf(resMerc.Description)
			if !emulation {
				return "", descrError, err
			} else {
				testnomsessii = testnomsessii + 1
				resMerc.SessionKey = "эмуляция" + strconv.Itoa(testnomsessii)
			}
		}
		sessionkey = resMerc.SessionKey
		//defer closesession(ipktt, port, sessionkey, loginfo)
	}
	answerbyteKKT, errStatusKKT := getStatusKKT(ipktt, port, sessionkey)
	if errStatusKKT != nil {
		descrError := "ошибка получения статуса ккт меркурий"
		Closesession(ipktt, port, &sessionkey)
		return "", descrError, errStatusKKT
	}
	errUnmarshKKT := json.Unmarshal(answerbyteKKT, &resMerc)
	if errUnmarshKKT != nil {
		descrError := fmt.Sprintf("ошибка распаковки ответа %v ккт меркурий", string(answerbyteKKT))
		Closesession(ipktt, port, &sessionkey)
		return "", descrError, errUnmarshKKT
	}
	if !resMerc.ShiftInfo.IsOpen {
		descrError := "смена не открыта"
		if consttypes.Logger != nil {
			consttypes.Logger.Println(descrError)
		}
		//return "", descrError, nil
	}
	if resMerc.Result != 0 {
		descrError := fmt.Sprintf("ккт меркурий не работает по причине %v", resMerc.Description)
		if !emulation {
			Closesession(ipktt, port, &sessionkey)
			err := errors.New(resMerc.Description)
			return "", descrError, err
		}
	}
	return sessionkey, "", nil
} //CheckStatsuConnectionKKT

func DissconnectMeruriy(ipktt string, port int, sessionkey string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	if sessionkey != "" {
		Closesession(ipktt, port, &sessionkey)
	}
	jsonmerc := []byte("{\"command\":\"ClosePorts\"}")
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		descrError := fmt.Sprintf("ошибка (%v) закрытия всех не активных портов для меркурия", err)
		return descrError, err
	}
	err = json.Unmarshal(buffAnsw, &resMerc)
	if err != nil {
		descrError := fmt.Sprintf("ошибка (%v) маршалинга результата закрытия закрытия всех не активных портов для меркурия", err)
		return descrError, err
	}
	if resMerc.Result != 0 {
		descrError := fmt.Sprintf("ошибка (%v) закрытия всех не активных портов для меркурий", resMerc.Description)
		err = fmt.Errorf(resMerc.Description)
		return descrError, err
	}
	return "", nil
}

func BreakAndClearProccessOfMarks(ipktt string, port int, comport int, sessionkey string, userint int, passwuser string) (string, error) {
	desckErrorBreak, errBreek := BreakProcCheckOfMark(ipktt, port, comport, sessionkey, userint, passwuser)
	desckErrorBreakClear, errClear := ClearTablesOfMarks(ipktt, port, comport, sessionkey, userint, passwuser)
	err := errors.Join(errBreek, errClear)
	return desckErrorBreak + desckErrorBreakClear, err
}

func BreakProcCheckOfMark(ipktt string, port int, comport int, sessionkey string, userint int, passwuser string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	if sessionkey == "" {
		answer, err := opensession(ipktt, port, comport, userint, passwuser)
		if err != nil {
			descrError := "ошибка при подключении к ккт меркурий"
			return descrError, err
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := "ошибка при подключении к ккт меркурий"
			return descrError, err
		}
		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := "ошибка при подключении к ккт меркурий"
			err = fmt.Errorf(resMerc.Description)
			return descrError, err
		}
		sessionkey = resMerc.SessionKey
		defer Closesession(ipktt, port, &sessionkey)
	}
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"AbortMarkingCodeChecking\"}", sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		descrError := "ошибка прерывания проверки марок"
		return descrError, err
	}
	err = json.Unmarshal(buffAnsw, &resMerc)
	if err != nil {
		descrError := "ошибка прерывания проверки марок"
		return descrError, err
	}
	descrError := resMerc.Description
	return descrError, nil
} //breakProcCheckOfMark

func ClearTablesOfMarks(ipktt string, port int, comport int, sessionkey string, userint int, passwuser string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	if sessionkey == "" {
		answer, err := opensession(ipktt, port, comport, userint, passwuser)
		if err != nil {
			descrError := "ошибка при подключении к ккт меркурий"
			return descrError, err
		}
		err = json.Unmarshal(answer, &resMerc)
		if err != nil {
			descrError := "ошибка при подключении к ккт меркурий"
			return descrError, err
		}
		if resMerc.Result != 0 || resMerc.SessionKey == "" {
			descrError := "ошибка при подключении к ккт меркурий"
			err = fmt.Errorf(resMerc.Description)
			return descrError, err
		}
		sessionkey = resMerc.SessionKey
		defer Closesession(ipktt, port, &sessionkey)
	}
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"ClearMarkingCodeValidationTable\"}", sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		descrError := "ошибка очистки таблицы марок"
		return descrError, err
	}
	err = json.Unmarshal(buffAnsw, &resMerc)
	if err != nil {
		descrError := "ошибка очистки таблицы марок"
		return descrError, err
	}
	descrError := resMerc.Description
	return descrError, nil
} //BreakProccessMarkAndClearTablesOfMarks

// ////////////////////
func getStatusServerKKT(ipktt string, port int) ([]byte, error) {
	jsonmerc := []byte("{\"command\":\"GetDriverInfo\"}")
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		return nil, err
	}
	return buffAnsw, nil
} //getStatusServerKKT

/*func getInfoKKT(ipktt string, port int, sessionkey string) ([]byte, error) {
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"GetCommonInfo\"}", sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		return nil, err
	}
	return buffAnsw, nil
} //getStatusKKT*/

func getJSONBeginProcessMarkCheck(mark string, measureunit int, sessionkey string) ([]byte, error) {
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"CheckMarkingCode\", \"mc\":\"%v\", \"plannedStatus\": 255, \"qty\": 10000, \"measureUnit\": %v}", sessionkey, mark, measureunit))
	return jsonmerc, nil
}

func SendCheckOfMark(ipktt string, port int, sessionkey, mark string, measureunit int) ([]byte, error) {
	jsonBeginProcMark, err := getJSONBeginProcessMarkCheck(mark, measureunit, sessionkey)
	if err != nil {
		return nil, err
	}
	return sendCommandTCPMerc(jsonBeginProcMark, ipktt, port)
}

func GetStatusOfChecking(ipktt string, port int, sessionkey string) ([]byte, error) {
	jsonOfStatusProcMark := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"GetMarkingCodeCheckResult\"}", sessionkey))
	return sendCommandTCPMerc(jsonOfStatusProcMark, ipktt, port)
}

func AcceptMark(ipktt string, port int, sessionkey string) ([]byte, error) {
	jsonOfStatusProcMark := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"AcceptMarkingCode\"}", sessionkey))
	return sendCommandTCPMerc(jsonOfStatusProcMark, ipktt, port)
}

func getStatusKKT(ipktt string, port int, sessionkey string) ([]byte, error) {
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"GetStatus\"}", sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		return nil, err
	}
	return buffAnsw, nil
} //getStatusKKT

func convertDocToMercHeader(checkdoc consttypes.TDocument, snoDefault int) (consttypes.TMercOpenCheck, error) {
	var checheaderkmerc consttypes.TMercOpenCheck
	checheaderkmerc.Command = "OpenCheck"
	checheaderkmerc.CheckType = 0
	if checkdoc.IsReturn {
		checheaderkmerc.CheckType = 1
	}
	if checkdoc.TaxationType == "" {
		if snoDefault == -1 {
			err := fmt.Errorf("не задан тип налогообложения")
			return checheaderkmerc, err
		}
	}
	checheaderkmerc.TaxSystem = snoDefault
	if checkdoc.TaxationType == "osn" {
		checheaderkmerc.TaxSystem = 0
	} else if checkdoc.TaxationType == "usnIncome" {
		checheaderkmerc.TaxSystem = 1
	} else if checkdoc.TaxationType == "usnIncomeOutcome" {
		checheaderkmerc.TaxSystem = 2
	} else if checkdoc.TaxationType == "esn" {
		checheaderkmerc.TaxSystem = 4
	} else if checkdoc.TaxationType == "patent" {
		checheaderkmerc.TaxSystem = 5
	}
	checheaderkmerc.CashierInfo.CashierName = checkdoc.Cashier
	return checheaderkmerc, nil
} //convertAtolToMercHeader

func convertDocPosToMercPos(pos consttypes.TItem, returnDoc bool) (consttypes.TMercPosition, error) {
	var mercPos consttypes.TMercPosition
	mercPos.Command = "AddGoods"
	mercPos.ProductName = pos.Name
	//mercPos.Qty = int(pos.Quantity * 10000)
	mercPos.Qty = pos.Quantity
	mercPos.MeasureUnit = 0
	mercPos.TaxCode = 6
	mercPos.PaymentFormCode = 4
	mercPos.ProductTypeCode = 1
	if pos.Mark != "" {
		mercPos.ProductTypeCode = 33
	}
	//mercPos.Price = int(pos.Price * 100)
	mercPos.Price = pos.Price
	if pos.Mark != "" {
		mercPos.McInfo = new(consttypes.TMcInfoMerc)
		mercPos.McInfo.Mc = pos.Mark
		if returnDoc {
			mercPos.McInfo.PlannedStatus = 3
		} else {
			mercPos.McInfo.PlannedStatus = 1
		}
		mercPos.McInfo.ProcessingMode = 0
	}
	return mercPos, nil
} //convertDocPosToMercPos

func convertDocMercCloseCheck(checkatol consttypes.TDocument) consttypes.TMercCloseCheck {
	var checclosekmerc consttypes.TMercCloseCheck
	checclosekmerc.Command = "CloseCheck"
	checclosekmerc.Payment.Cash = int(checkatol.Cash * 100)
	checclosekmerc.Payment.Ecash = int(checkatol.Ecash * 100)
	return checclosekmerc
} //convertDocMercCloseCheck

func sendCommandTCPMerc(bytesjson []byte, ip string, port int) ([]byte, error) {
	var buffAnsw []byte
	conn, err := net.DialTimeout("tcp", ip+":"+strconv.Itoa(port), 5*time.Second)
	if err != nil {
		descError := fmt.Sprintf("error: ошибка рукопожатия tcp %v\r\n", err)
		descError = descError + fmt.Sprintln("сервер ККТ не отвечает ККТ")
		err = fmt.Errorf("ошибка рукопожатия tcp %v\nсервер ККТ не отвечает ККТ", err)
		err = errors.Join(err, errors.New(descError))
		return buffAnsw, err
	}
	defer conn.Close()
	jsonBytes := bytesjson
	lenTCP := int32(len(jsonBytes))
	bytesLen := make([]byte, 4)
	bytesLen[3] = byte(lenTCP >> 0)
	bytesLen[2] = byte(lenTCP >> (1 * 8))
	bytesLen[1] = byte(lenTCP >> (2 * 8))
	bytesLen[0] = byte(lenTCP >> (3 * 8))
	var bufTCP bytes.Buffer
	_, err = bufTCP.Write(bytesLen)
	if err != nil {
		descError := fmt.Sprintf("error: ошибка записи в буфер данных длины пакета: %v\r\n", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	bufTCP.Write(jsonBytes)
	bufTCPReader := bytes.NewReader(bufTCP.Bytes())
	buffAnsw = make([]byte, 1024)
	var n int
	_, err = mustCopy(conn, bufTCPReader)
	if err != nil {
		descError := fmt.Sprintf("error: ошибка отправка tcp заароса серверу Мекрурия %v\r\n", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	n, err = conn.Read(buffAnsw)
	if err != nil {
		descError := fmt.Sprintf("error: ошибка получения ответа от сервера Меркурия  %v \r\n", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	fmt.Println(string(buffAnsw))
	return buffAnsw[4:n], nil
} //sendCommandTCPMerc

func mustCopy(dst io.Writer, src io.Reader) (int64, error) {
	count, err := io.Copy(dst, src)
	if err != nil {
		descError := fmt.Sprintf("ошибка копирования %v\r\n", err)
		fmt.Println(descError)
	}
	return count, err
} //mustCopy

func opensession(ipktt string, port int, comport int, userint int, passwuser string) ([]byte, error) {
	var jsonmerc []byte
	//jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"null\", \"command\":\"OpenSession\", \"portName\":\"COM%v\"}", comport))
	//jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":null, \"command\":\"OpenSession\", \"portName\":\"COM%v\"}", comport))
	if (userint != 0) || (passwuser != "") {
		jsonmerc = []byte(fmt.Sprintf("{\"sessionKey\":null, \"command\":\"OpenSession\", \"portName\":\"COM%v\", \"model\":\"185F\", \"userNumber\": %v,\"userPassword\": \"%v\", \"debug\": true, \"logPath\": \"c:\\\\logs\\\\\"}", comport, userint, passwuser))
	} else {
		jsonmerc = []byte(fmt.Sprintf("{\"sessionKey\":null, \"command\":\"OpenSession\", \"portName\":\"COM%v\", \"model\":\"185F\", \"debug\": true, \"logPath\": \"c:\\\\logs\\\\\"}", comport))
	}

	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду OpenSession: %v", string(jsonmerc))
	}

	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if err != nil {
		descError := fmt.Sprintf("ошибка (%v) открытия сессии для кассы меркурий", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	return buffAnsw, nil
} //opensession

func opencheck(ipktt string, port int, headercheckjson []byte) ([]byte, error) {
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду opencheck: %v", string(headercheckjson))
	}
	buffAnsw, err := sendCommandTCPMerc(headercheckjson, ipktt, port)
	if err != nil {
		descError := fmt.Sprintf("ошибка (%v) открытия чека для кассы меркурий", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	return buffAnsw, nil
} //opencheck

func addpos(ipktt string, port int, posjson []byte) ([]byte, error) {
	buffAnsw, err := sendCommandTCPMerc(posjson, ipktt, port)
	if err != nil {
		descError := fmt.Sprintf("ошибка (%v) добавления позиции для кассы меркурий", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	return buffAnsw, nil
}

func closecheck(ipktt string, port int, forclosedatamerc []byte) ([]byte, error) {
	buffAnsw, err := sendCommandTCPMerc(forclosedatamerc, ipktt, port)
	if err != nil {
		descError := fmt.Sprintf("ошибка (%v) закрытия чека для кассы меркурий", err)
		fmt.Println(descError)
		return buffAnsw, err
	}
	return buffAnsw, nil
} //closecheck

func cancelcheck(ipktt string, port int, sessionkey *string) ([]byte, error) {
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Отправляем команду ResetCheck: %v", *sessionkey)
	}
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"ResetCheck\"}", *sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Ответ на команду ResetCheck: %v", string(buffAnsw))
	}
	if err != nil {
		descError := fmt.Sprintf("ошибка (%v) отмены чека для кассы меркурий", err)
		if consttypes.Logger != nil {
			consttypes.Logger.Println(descError)
		}
		fmt.Println(descError)
		return buffAnsw, err
	}
	return buffAnsw, nil
} //closecheck

func Closesession(ipktt string, port int, sessionkey *string) (string, error) {
	var resMerc consttypes.TAnswerMercur
	jsonmerc := []byte(fmt.Sprintf("{\"sessionKey\":\"%v\", \"command\":\"CloseSession\"}", *sessionkey))
	buffAnsw, err := sendCommandTCPMerc(jsonmerc, ipktt, port)
	*sessionkey = ""
	if err != nil {
		descrError := fmt.Sprintf("ошибка (%v) закрытия сессии для кассы меркурий", err)
		fmt.Println(descrError)
		return descrError, err
	}
	err = json.Unmarshal(buffAnsw, &resMerc)
	if err != nil {
		descrError := fmt.Sprintf("ошибка (%v) маршалинга результата закрытия сессии для кассы меркурий", err)
		fmt.Println(descrError)
		return descrError, err
	}
	if resMerc.Result != 0 {
		descrError := fmt.Sprintf("ошибка (%v) закрытия сессии для кассы меркурий", resMerc.Description)
		fmt.Println(descrError)
		err = fmt.Errorf(resMerc.Description)
		return descrError, err
	}
	return "", nil
} //closesession

func successCommand(resulJson string) bool {
	res := true
	indOsh := strings.Contains(resulJson, "ошибка")
	indErr := strings.Contains(resulJson, "error")
	if indErr || indOsh {
		res = false
	}
	return res
} //successCommand
