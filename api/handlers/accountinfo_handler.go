package handlers

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/seeleteam/scan-api/database"
	"github.com/seeleteam/scan-api/log"
)

const (
	shardCount        = 20
	maxShowAccountNum = 10000
	txCount           = 25
	//exclude divide zero problem
	remianTotalBalance = 1
)

//AccountTbl represents an account list ordered by account balance
type AccountTbl struct {
	shardNumber  int
	DBClient     BlockInfoDB
	accountTbl   []*database.DBAccount
	accMutex     sync.RWMutex
	totalBalance int64
}

//ProcessGAccountTable process global account table
func (a *AccountTbl) ProcessGAccountTable() {
	temp, err := a.DBClient.GetAccountsByShardNumber(a.shardNumber, maxShowAccountNum)
	if err != nil {
		log.Error("[DB] err : %v", err)
	} else {
		a.accMutex.Lock()
		a.accountTbl = temp
		a.accMutex.Unlock()
	}
}

//GetAccountCnt get the length of account table
func (a *AccountTbl) GetAccountCnt() int {
	a.accMutex.RLock()
	size := len(a.accountTbl)
	a.accMutex.RUnlock()
	return size
}

//GetAccountsByIdx get a transaction list from mongo by time period
func (a *AccountTbl) GetAccountsByIdx(begin uint64, end uint64) []*database.DBAccount {
	a.accMutex.RLock()
	if end > uint64(len(a.accountTbl)) {
		end = uint64(len(a.accountTbl)) - 1
	}

	retAccounts := a.accountTbl[begin:end]
	a.accMutex.RUnlock()
	return retAccounts
}

//getAccountsByBeginAndEnd
func (a *AccountTbl) getAccountsByBeginAndEnd(begin, end uint64) []*RetSimpleAccountInfo {
	var accounts []*RetSimpleAccountInfo

	dbAccounts := a.GetAccountsByIdx(begin, end)

	for i := 0; i < len(dbAccounts); i++ {
		data := dbAccounts[i]

		simpleAccount := createRetSimpleAccountInfo(data, a.totalBalance)
		simpleAccount.Rank = i + 1
		accounts = append(accounts, simpleAccount)
	}

	return accounts
}

//AccountHandler handle all account request
type AccountHandler struct {
	accTbls  []*AccountTbl
	DBClient BlockInfoDB
}

//NewAccHandler return an accounthandler to handler account request
func NewAccHandler(DBClient BlockInfoDB) *AccountHandler {
	var accTbls []*AccountTbl
	for i := 1; i <= shardCount; i++ {
		accTbl := &AccountTbl{shardNumber: i, DBClient: DBClient}
		accTbls = append(accTbls, accTbl)
	}

	ret := &AccountHandler{
		accTbls:  accTbls,
		DBClient: DBClient,
	}

	ret.updateImpl()
	return ret
}

func (h *AccountHandler) updateImpl() {

	for i := 1; i < shardCount; i++ {
		h.accTbls[i-1].ProcessGAccountTable()
	}

	totalBalances, err := h.DBClient.GetTotalBalance()
	if err != nil {
		log.Error("[DB] err : %v", err)
	}

	for i := 0; i < shardCount; i++ {
		if v, exist := totalBalances[i+1]; exist != false {
			h.accTbls[i].totalBalance = v
		} else {
			h.accTbls[i].totalBalance = remianTotalBalance
		}
	}
}

//Update Update account list every 5 secs
func (h *AccountHandler) Update() {
	for {
		now := time.Now()
		// calcuate next zero hour
		next := now.Add(time.Second * 5)
		t := time.NewTimer(next.Sub(now))
		<-t.C

		h.updateImpl()
	}
}

//GetAccounts handler for get account list
func (h *AccountHandler) GetAccounts() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, _ := strconv.ParseUint(c.Query("p"), 10, 64)
		ps, _ := strconv.ParseUint(c.Query("ps"), 10, 64)
		s, _ := strconv.ParseInt(c.Query("s"), 10, 64)
		if ps == 0 {
			ps = blockItemNumsPrePage
		} else if ps > maxItemNumsPrePage {
			ps = maxItemNumsPrePage
		}

		if p >= 1 {
			p--
		}

		if s <= 0 {
			s = 1
		}
		shardNumber := int(s)

		if shardNumber < 1 || shardNumber > 20 {
			responseError(c, errParamInvalid, http.StatusBadRequest, apiParmaInvalid)
			return
		}

		accTbl := h.accTbls[shardNumber-1]
		accCnt := accTbl.GetAccountCnt()

		page, begin, end := getBeginAndEndByPage(uint64(accCnt), p, ps)
		accounts := accTbl.getAccountsByBeginAndEnd(begin, end)

		c.JSON(http.StatusOK, gin.H{
			"code":    apiOk,
			"message": "",
			"data": gin.H{
				"pageInfo": gin.H{
					"totalCount":   accCnt,
					"begin":        begin,
					"end":          end,
					"curPage":      page + 1,
					"totalBalance": accTbl.totalBalance,
				},
				"list": accounts,
			},
		})
	}
}

//GetAccountByAddressImpl use account info, account tx list and account pending tx list to assembly account information
func (h *AccountHandler) GetAccountByAddressImpl(address string) *RetDetailAccountInfo {
	dbClinet := h.DBClient

	data, err := dbClinet.GetAccountByAddress(address)
	if err != nil {
		return nil
	}

	if data.AccType != 0 {
		return nil
	}

	txs, err := dbClinet.GetTxsByAddresss(address, txCount)
	if err != nil {
		return nil
	}

	pengdingTxs, err := dbClinet.GetPendingTxsByAddress(address)
	if err != nil {
		return nil
	}

	txs = append(pengdingTxs, txs...)

	var ttBalance int64
	if data.ShardNumber >= 1 && data.ShardNumber <= shardCount {
		ttBalance = h.accTbls[data.ShardNumber-1].totalBalance
	}

	detailAccount := createRetDetailAccountInfo(data, txs, ttBalance)
	return detailAccount
}

//GetAccountByAddress get account detail info by address
func (h *AccountHandler) GetAccountByAddress() gin.HandlerFunc {
	return func(c *gin.Context) {
		address := c.Query("address")
		// if len(address) != addressLength {
		// 	responseError(c, errParamInvalid, http.StatusBadRequest, apiParmaInvalid)
		// 	return
		// }

		detailAccount := h.GetAccountByAddressImpl(address)

		c.JSON(http.StatusOK, gin.H{
			"code":    apiOk,
			"message": "",
			"data":    detailAccount,
		})

	}
}
