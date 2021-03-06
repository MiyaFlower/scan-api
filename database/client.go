package database

import (
	"errors"
	"strconv"

	"github.com/seeleteam/scan-api/log"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	blockTbl     = "block"
	txTbl        = "transaction"
	accTbl       = "account"
	pendingTxTbl = "pendingtx"

	chartTxTbl              = "chart_transhistory"
	chartHashRateTbl        = "chart_hashrate"
	chartBlockDifficultyTbl = "chart_blockdifficulty"
	chartBlockAvgTimeTbl    = "chart_blockavgtime"
	chartBlockTbl           = "chart_block"
	chartAddressTbl         = "chart_address"
	chartSingleAddressTbl   = "chart_single_address"
	chartTopMinerRankTbl    = "chart_topminer"

	nodeInfoTbl = "nodeinfo"
)

var (
	mgoSession *mgo.Session
	//db connect error
	errDBConnect = errors.New("could not connect to database")
)

//Client warpper for mongodb interactive
type Client struct {
	mgo         *mgo.Session
	dbName      string
	connUrl     string
	shardNumber int
}

//NewDBClient reuturn an DB client
func NewDBClient(dbName, connUrl string, shardNumber int) *Client {
	mgo := getSession(connUrl)
	if mgo == nil {
		return nil
	}

	return &Client{
		mgo:         mgo,
		dbName:      dbName,
		connUrl:     connUrl,
		shardNumber: shardNumber,
	}
}

//getSession return an mongo db instance by connurl
func getSession(connUrl string) *mgo.Session {
	mgoSession, err := mgo.Dial(connUrl)
	if err != nil {
		log.Error("[DB] err : %v", err)
		return nil
	}
	return mgoSession
}

func (c *Client) getDBConnection() *mgo.Session {
	if c.mgo == nil {
		c.mgo = getSession(c.connUrl)
		return c.mgo.Clone()
	}
	return c.mgo.Clone()
}

//withCollection perform an database query
func (c *Client) withCollection(collection string, s func(*mgo.Collection) error) error {
	session := c.getDBConnection()
	defer func() {
		if session != nil {
			session.Close()
		}
	}()
	if session != nil {
		c := session.DB(c.dbName).C(collection)
		err := s(c)
		processDataBaseError(err)
		return err
	}
	log.Error("[DB] err : could not connect to db, host is %s", c.connUrl)
	return errDBConnect
}

//dropCollection test use remove the tbl
func (c *Client) dropCollection(tbl string) error {
	session := c.getDBConnection()
	if session != nil {
		c := session.DB(c.dbName).C(tbl)
		err := c.DropCollection()
		processDataBaseError(err)
		return err
	}
	log.Error("[DB] err : could not connect to db, host is %s", c.connUrl)
	return errDBConnect
}

//AddBlock insert a block into database
func (c *Client) AddBlock(b *DBBlock) error {
	query := func(c *mgo.Collection) error {
		return c.Insert(b)
	}
	err := c.withCollection(blockTbl, query)
	return err
}

//RemoveBlock test use  remove block by height from database
func (c *Client) RemoveBlock(height uint64) error {
	query := func(c *mgo.Collection) error {
		return c.Remove(bson.M{"height": height})
	}
	err := c.withCollection(blockTbl, query)
	return err
}

//GetBlockByHeight get block from mongo by block height
func (c *Client) GetBlockByHeight(shardNumber int, height uint64) (*DBBlock, error) {
	b := new(DBBlock)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"height": height, "shardNumber": shardNumber}).One(b)
	}
	err := c.withCollection(blockTbl, query)
	return b, err
}

//GetBlockByHash get a block from mongo by block header hash
func (c *Client) GetBlockByHash(hash string) (*DBBlock, error) {
	b := new(DBBlock)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"headHash": hash}).One(b)
	}
	err := c.withCollection(blockTbl, query)
	return b, err
}

//GetBlocksByHeight get a block list from mongo by height range
func (c *Client) GetBlocksByHeight(shardNumber int, begin uint64, end uint64) ([]*DBBlock, error) {
	var blocks []*DBBlock

	query := func(c *mgo.Collection) error {

		return c.Find(bson.M{"height": bson.M{"$gte": begin, "$lt": end}, "shardNumber": shardNumber}).Sort("-height").All(&blocks)
	}
	err := c.withCollection(blockTbl, query)
	return blocks, err
}

//GetBlocksByTime get a block list from mongo by time period
func (c *Client) GetBlocksByTime(shardNumber int, beginTime, endTime int64) ([]*DBBlock, error) {
	var blocks []*DBBlock

	query := func(c *mgo.Collection) error {

		return c.Find(bson.M{"timestamp": bson.M{"$gte": beginTime, "$lte": endTime}, "shardNumber": shardNumber}).All(&blocks)
	}
	err := c.withCollection(blockTbl, query)
	return blocks, err
}

//GetBlockHeight get row count of block table from mongo
func (c *Client) GetBlockHeight(shardNumber int) (uint64, error) {
	var blockCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"shardNumber": shardNumber}).Count()
		blockCnt = uint64(temp)
		return err
	}
	err := c.withCollection(blockTbl, query)
	return blockCnt, err
}

//AddTx insert a transaction into mongo
func (c *Client) AddTx(tx *DBTx) error {
	query := func(c *mgo.Collection) error {
		return c.Insert(tx)
	}
	err := c.withCollection(txTbl, query)
	return err
}

//AddPendingTx insert a pending transaction into mongo
func (c *Client) AddPendingTx(tx *DBTx) error {
	query := func(c *mgo.Collection) error {
		return c.Insert(tx)
	}
	err := c.withCollection(pendingTxTbl, query)
	return err
}

//RemoveAllPendingTxs remove all pending transactions
func (c *Client) RemoveAllPendingTxs() error {
	query := func(c *mgo.Collection) error {
		_, err := c.RemoveAll(nil)
		return err
	}
	err := c.withCollection(pendingTxTbl, query)
	return err
}

//removeTx test use  remove tx by index from database
func (c *Client) removeTx(idx uint64) error {
	query := func(c *mgo.Collection) error {
		return c.Remove(bson.M{"idx": strconv.FormatUint(idx, 10)})
	}
	err := c.withCollection(txTbl, query)
	return err
}

//RemoveTxs Txs by block height
func (c *Client) RemoveTxs(blockHeight uint64) error {
	query := func(c *mgo.Collection) error {
		return c.Remove(bson.M{"block": strconv.FormatUint(blockHeight, 10)})
	}
	err := c.withCollection(txTbl, query)
	return err
}

//GetTxByIdx get transaction from mongo by idx
func (c *Client) GetTxByIdx(idx uint64) (*DBTx, error) {
	tx := new(DBTx)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"idx": idx}).One(tx)
	}
	err := c.withCollection(txTbl, query)
	return tx, err
}

//GetTxsByIdx get a transaction list from mongo by time period
func (c *Client) GetTxsByIdx(shardNumber int, begin uint64, end uint64) ([]*DBTx, error) {
	var trans []*DBTx
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardNumber": shardNumber, "idx": bson.M{"$gte": begin, "$lt": end}}).Sort("-idx").All(&trans)
	}
	err := c.withCollection(txTbl, query)
	return trans, err
}

//GetPendingTxsByIdx get a transaction list from mongo by time period
func (c *Client) GetPendingTxsByIdx(shardNumber int, begin uint64, end uint64) ([]*DBTx, error) {
	var trans []*DBTx
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardNumber": shardNumber, "idx": bson.M{"$gte": begin, "$lt": end}}).Sort("-idx").All(&trans)
	}
	err := c.withCollection(pendingTxTbl, query)
	return trans, err
}

//GetTxByHash get transaction info by hash from mongo
func (c *Client) GetTxByHash(hash string) (*DBTx, error) {
	tx := new(DBTx)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"hash": hash}).One(tx)
	}
	err := c.withCollection(txTbl, query)
	return tx, err
}

//GetPendingTxByHash
func (c *Client) GetPendingTxByHash(hash string) (*DBTx, error) {
	tx := new(DBTx)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"hash": hash}).One(tx)
	}
	err := c.withCollection(pendingTxTbl, query)
	return tx, err
}

//GetTxCnt get row count of transaction table from mongo
func (c *Client) GetTxCnt() (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(txTbl, query)
	return txCnt, err
}

//GetBlockCnt get row count of transaction table from mongo
func (c *Client) GetBlockCnt() (uint64, error) {
	var blockCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Count()
		blockCnt = uint64(temp)
		return err
	}
	err := c.withCollection(blockTbl, query)
	return blockCnt, err
}

//GetAccountCnt get account count
func (c *Client) GetAccountCnt() (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"accType": 0}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(accTbl, query)
	return txCnt, err
}

//GetContractCnt get contract count
func (c *Client) GetContractCnt() (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"accType": 1}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(accTbl, query)
	return txCnt, err
}

//GetAccountCntByShardNumber get contract count
func (c *Client) GetAccountCntByShardNumber(shardNumber int) (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"accType": 0, "shardnumber": shardNumber}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(accTbl, query)
	return txCnt, err
}

//GetContractCntByShardNumber get contract count
func (c *Client) GetContractCntByShardNumber(shardNumber int) (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"accType": 1, "shardnumber": shardNumber}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(accTbl, query)
	return txCnt, err
}

//GetTxCntByShardNumber get tx count by shardNumber
func (c *Client) GetTxCntByShardNumber(shardNumber int) (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"shardNumber": shardNumber}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(txTbl, query)
	return txCnt, err
}

func (c *Client) GetPendingTxCntByShardNumber(shardNumber int) (uint64, error) {
	var txCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"shardNumber": shardNumber}).Count()
		txCnt = uint64(temp)
		return err
	}
	err := c.withCollection(pendingTxTbl, query)
	return txCnt, err
}

//GetTxCntByShardNumberAndAddress get tx count for the account
func (c *Client) GetTxCntByShardNumberAndAddress(shardNumber int, address string) (int64, error) {
	var txCnt int64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int

		temp, err = c.Find(bson.M{"shardNumber": shardNumber, "$or": []bson.M{bson.M{"from": address}, bson.M{"to": address}}}).Count()
		txCnt = int64(temp)
		return err
	}
	err := c.withCollection(txTbl, query)
	return txCnt, err
}

//GetMinedBlocksCntByShardNumberAndAddress
func (c *Client) GetMinedBlocksCntByShardNumberAndAddress(shardNumber int, address string) (int64, error) {
	var blockCnt int64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int

		temp, err = c.Find(bson.M{"shardNumber": shardNumber, "creator": address}).Count()
		blockCnt = int64(temp)
		return err
	}
	err := c.withCollection(blockTbl, query)
	return blockCnt, err
}

//removeAccount test use  remove account by address from database
func (c *Client) removeAccount(address string) error {
	query := func(c *mgo.Collection) error {
		return c.Remove(bson.M{"address": address})
	}
	err := c.withCollection(accTbl, query)
	return err
}

//GetTxsByAddresss return a tx list by address
func (c *Client) GetTxsByAddresss(address string, max int) ([]*DBTx, error) {
	var trans []*DBTx
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"$or": []bson.M{bson.M{"from": address}, bson.M{"to": address}, bson.M{"contractAddress": address}}}).Sort("-timestamp").Limit(max).All(&trans)
	}
	err := c.withCollection(txTbl, query)
	return trans, err
}

//GetPendingTxsByAddress return a pengding tx list by address
func (c *Client) GetPendingTxsByAddress(address string) ([]*DBTx, error) {
	var trans []*DBTx
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"$or": []bson.M{bson.M{"from": address}, bson.M{"to": address}, bson.M{"contractAddress": address}}}).Sort("-timestamp").All(&trans)
	}
	err := c.withCollection(pendingTxTbl, query)
	return trans, err
}

//GetAccountByAddress get an dbaccount by account address
func (c *Client) GetAccountByAddress(address string) (*DBAccount, error) {
	account := new(DBAccount)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"address": address}).One(account)
	}
	err := c.withCollection(accTbl, query)
	return account, err
}

//AddAccount insert an account into database
func (c *Client) AddAccount(account *DBAccount) error {
	query := func(c *mgo.Collection) error {
		return c.Insert(account)
	}
	err := c.withCollection(accTbl, query)
	return err
}

//UpdateAccount update account
func (c *Client) UpdateAccount(account *DBAccount) error {
	query := func(c *mgo.Collection) error {
		_, err := c.Upsert(bson.M{"address": account.Address}, account)
		return err
	}
	err := c.withCollection(accTbl, query)
	if err != nil {
		return err
	}

	return err
}

//UpdateAccountMinedBlock update field mined block in the account info
func (c *Client) UpdateAccountMinedBlock(address string, mined int64) error {
	query := func(c *mgo.Collection) error {
		return c.Update(bson.M{"address": address},
			bson.M{"$set": bson.M{
				"mined": mined,
			}})
	}
	err := c.withCollection(accTbl, query)
	return err
}

//GetAccountsByShardNumber get an dbaccount list sort by balance
func (c *Client) GetAccountsByShardNumber(shardNumber int, max int) ([]*DBAccount, error) {
	var accounts []*DBAccount
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"accType": 0, "shardNumber": shardNumber}).Sort("-balance").Limit(max).All(&accounts)
	}
	err := c.withCollection(accTbl, query)
	return accounts, err
}

//GetContractsByShardNumber
func (c *Client) GetContractsByShardNumber(shardNumber int, max int) ([]*DBAccount, error) {
	var accounts []*DBAccount
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"accType": 1, "shardNumber": shardNumber}).Sort("-timestamp").Limit(max).All(&accounts)
	}
	err := c.withCollection(accTbl, query)
	return accounts, err
}

//GetTotalBalance return the sum of all account
func (c *Client) GetTotalBalance() (map[int]int64, error) {
	totalBalance := make(map[int]int64)
	query := func(c *mgo.Collection) error {

		job := &mgo.MapReduce{
			Map: "function() { emit(this.shardNumber, this.balance) }",
			Reduce: `function(key, values) {
						return Array.sum(values)
					}`,
		}
		var result []struct {
			Id    int "_id"
			Value int64
		}
		_, err := c.Find(nil).MapReduce(job, &result)
		if err != nil {
			return err
		}
		for _, item := range result {
			totalBalance[item.Id] = item.Value
		}

		return err
	}
	err := c.withCollection(accTbl, query)
	return totalBalance, err
}

//processDataBaseError shutdown database connection and log it
func processDataBaseError(err error) {
	if err == nil || err == mgo.ErrNotFound || err == mgo.ErrCursor {
		return
	}

	log.Error("[DB] err : %v", err)
}

//AddOneDayTransInfo insert one dya transaction info into mongo
func (c *Client) AddOneDayTransInfo(shardNumber int, t *DBOneDayTxInfo) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartTxTbl, query)
	return err
}

//GetOneDayTransInfo get one day transaction info from mongo by zero hour timestamp
func (c *Client) GetOneDayTransInfo(shardNumber int, zeroTime int64) (*DBOneDayTxInfo, error) {
	oneDayTransInfo := new(DBOneDayTxInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayTransInfo)
	}
	err := c.withCollection(chartTxTbl, query)
	return oneDayTransInfo, err
}

//GetTransInfoChart get all rows int the transhistory table
func (c *Client) GetTransInfoChart() ([]*DBOneDayTxInfo, error) {
	var oneDayTrans []*DBOneDayTxInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayTrans)
	}
	err := c.withCollection(chartTxTbl, query)
	return oneDayTrans, err
}

func (c *Client) GetTransInfoChartByShardNumber(shardNumber int) ([]*DBOneDayTxInfo, error) {
	var oneDayTrans []*DBOneDayTxInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayTrans)
	}
	err := c.withCollection(chartTxTbl, query)
	return oneDayTrans, err
}

//AddOneDayHashRate insert one dya hashrate info into mongo
func (c *Client) AddOneDayHashRate(shardNumber int, t *DBOneDayHashRate) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartHashRateTbl, query)
	return err
}

//GetOneDayHashRate get one day hashrate info from mongo by zero hour timestamp
func (c *Client) GetOneDayHashRate(shardNumber int, zeroTime int64) (*DBOneDayHashRate, error) {
	oneDayHashRate := new(DBOneDayHashRate)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayHashRate)
	}
	err := c.withCollection(chartHashRateTbl, query)
	return oneDayHashRate, err
}

//GetHashRateChart get all rows int the hashrate table
func (c *Client) GetHashRateChart() ([]*DBOneDayHashRate, error) {
	var oneDayHashRates []*DBOneDayHashRate
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayHashRates)
	}
	err := c.withCollection(chartHashRateTbl, query)
	return oneDayHashRates, err
}

//GetHashRateChartByShardNumber get ratechart by shardnumber
func (c *Client) GetHashRateChartByShardNumber(shardNumber int) ([]*DBOneDayHashRate, error) {
	var oneDayHashRates []*DBOneDayHashRate
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayHashRates)
	}
	err := c.withCollection(chartHashRateTbl, query)
	return oneDayHashRates, err
}

//AddOneDayBlockDifficulty insert one dya avg block difficulty info into mongo
func (c *Client) AddOneDayBlockDifficulty(shardNumber int, t *DBOneDayBlockDifficulty) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartBlockDifficultyTbl, query)
	return err
}

//GetOneDayBlockDifficulty get one day hashrate info from mongo by zero hour timestamp
func (c *Client) GetOneDayBlockDifficulty(shardNumber int, zeroTime int64) (*DBOneDayBlockDifficulty, error) {
	oneDayBlockDifficulty := new(DBOneDayBlockDifficulty)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayBlockDifficulty)
	}
	err := c.withCollection(chartBlockDifficultyTbl, query)
	return oneDayBlockDifficulty, err
}

//GetOneDayBlockDifficultyChart get all rows int the hashrate table
func (c *Client) GetOneDayBlockDifficultyChart() ([]*DBOneDayBlockDifficulty, error) {
	var oneDayBlockDifficulties []*DBOneDayBlockDifficulty
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayBlockDifficulties)
	}
	err := c.withCollection(chartBlockDifficultyTbl, query)
	return oneDayBlockDifficulties, err
}

func (c *Client) GetOneDayBlockDifficultyChartByShardNumber(shardNumber int) ([]*DBOneDayBlockDifficulty, error) {
	var oneDayBlockDifficulties []*DBOneDayBlockDifficulty
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayBlockDifficulties)
	}
	err := c.withCollection(chartBlockDifficultyTbl, query)
	return oneDayBlockDifficulties, err
}

//AddOneDayBlockAvgTime insert one dya avg block time info into mongo
func (c *Client) AddOneDayBlockAvgTime(shardNumber int, t *DBOneDayBlockAvgTime) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartBlockAvgTimeTbl, query)
	return err
}

//GetOneDayBlockAvgTime get one day avg block time info from mongo by zero hour timestamp
func (c *Client) GetOneDayBlockAvgTime(shardNumber int, zeroTime int64) (*DBOneDayBlockAvgTime, error) {
	oneDayBlockAvgTime := new(DBOneDayBlockAvgTime)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayBlockAvgTime)
	}
	err := c.withCollection(chartBlockAvgTimeTbl, query)
	return oneDayBlockAvgTime, err
}

//GetOneDayBlockAvgTimeChart get all rows int the hashrate table
func (c *Client) GetOneDayBlockAvgTimeChart() ([]*DBOneDayBlockAvgTime, error) {
	var oneDayBlockAvgTimes []*DBOneDayBlockAvgTime
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayBlockAvgTimes)
	}
	err := c.withCollection(chartBlockAvgTimeTbl, query)
	return oneDayBlockAvgTimes, err
}

func (c *Client) GetOneDayBlockAvgTimeChartByShardNumber(shardNumber int) ([]*DBOneDayBlockAvgTime, error) {
	var oneDayBlockAvgTimes []*DBOneDayBlockAvgTime
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayBlockAvgTimes)
	}
	err := c.withCollection(chartBlockAvgTimeTbl, query)
	return oneDayBlockAvgTimes, err
}

//AddOneDayBlock insert one dya block info into mongo
func (c *Client) AddOneDayBlock(shardNumber int, t *DBOneDayBlockInfo) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartBlockTbl, query)
	return err
}

//GetOneDayBlock get one day block info from mongo by zero hour timestamp
func (c *Client) GetOneDayBlock(shardNumber int, zeroTime int64) (*DBOneDayBlockInfo, error) {
	oneDayBlock := new(DBOneDayBlockInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayBlock)
	}
	err := c.withCollection(chartBlockTbl, query)
	return oneDayBlock, err
}

//GetOneDayBlocksChart get all rows int the hashrate table
func (c *Client) GetOneDayBlocksChart() ([]*DBOneDayBlockInfo, error) {
	var oneDayBlocks []*DBOneDayBlockInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayBlocks)
	}
	err := c.withCollection(chartBlockTbl, query)
	return oneDayBlocks, err
}

func (c *Client) GetOneDayBlocksChartByShardNumber(shardNumber int) ([]*DBOneDayBlockInfo, error) {
	var oneDayBlocks []*DBOneDayBlockInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayBlocks)
	}
	err := c.withCollection(chartBlockTbl, query)
	return oneDayBlocks, err
}

//AddOneDayAddress insert one dya block info into mongo
func (c *Client) AddOneDayAddress(shardNumber int, t *DBOneDayAddressInfo) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartAddressTbl, query)
	return err
}

//GetOneDayAddress get one day block info from mongo by zero hour timestamp
func (c *Client) GetOneDayAddress(shardNumber int, zeroTime int64) (*DBOneDayAddressInfo, error) {
	oneDayAddress := new(DBOneDayAddressInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"timestamp": zeroTime, "shardnumber": shardNumber}).One(oneDayAddress)
	}
	err := c.withCollection(chartAddressTbl, query)
	return oneDayAddress, err
}

//GetOneDayAddressesChart get all rows int the address table
func (c *Client) GetOneDayAddressesChart() ([]*DBOneDayAddressInfo, error) {
	var oneDayAddresses []*DBOneDayAddressInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).Sort("timestamp").All(&oneDayAddresses)
	}
	err := c.withCollection(chartAddressTbl, query)
	return oneDayAddresses, err
}

func (c *Client) GetOneDayAddressesChartByShardNumber(shardNumber int) ([]*DBOneDayAddressInfo, error) {
	var oneDayAddresses []*DBOneDayAddressInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).Sort("timestamp").All(&oneDayAddresses)
	}
	err := c.withCollection(chartAddressTbl, query)
	return oneDayAddresses, err
}

//AddOneDaySingleAddressInfo insert one dya single address info into mongo
func (c *Client) AddOneDaySingleAddressInfo(shardNumber int, t *DBOneDaySingleAddressInfo) error {
	t.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(t)
	}
	err := c.withCollection(chartSingleAddressTbl, query)
	return err
}

//GetOneDaySingleAddressInfo get one day block info from mongo by zero hour timestamp
func (c *Client) GetOneDaySingleAddressInfo(shardNumber int, address string) (*DBOneDaySingleAddressInfo, error) {
	oneDaySingleAddress := new(DBOneDaySingleAddressInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"address": address, "shardnumber": shardNumber}).One(oneDaySingleAddress)
	}
	err := c.withCollection(chartSingleAddressTbl, query)
	return oneDaySingleAddress, err
}

//RemoveTopMinerInfo remove last 7 days top miner info
func (c *Client) RemoveTopMinerInfo() error {
	query := func(c *mgo.Collection) error {
		return c.DropCollection()
	}
	err := c.withCollection(chartTopMinerRankTbl, query)
	return err
}

//AddTopMinerInfo add top miner rank info into database
func (c *Client) AddTopMinerInfo(shardNumber int, rankInfo *DBMinerRankInfo) error {
	rankInfo.ShardNumber = shardNumber
	query := func(c *mgo.Collection) error {
		return c.Insert(rankInfo)
	}
	err := c.withCollection(chartTopMinerRankTbl, query)
	return err
}

//GetTopMinerChart get all rows int the address table
func (c *Client) GetTopMinerChart() ([]*DBMinerRankInfo, error) {
	var topMinerInfo []*DBMinerRankInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).All(&topMinerInfo)
	}
	err := c.withCollection(chartTopMinerRankTbl, query)
	return topMinerInfo, err
}

func (c *Client) GetTopMinerChartByShardNumber(shardNumber int) ([]*DBMinerRankInfo, error) {
	var topMinerInfo []*DBMinerRankInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardnumber": shardNumber}).All(&topMinerInfo)
	}
	err := c.withCollection(chartTopMinerRankTbl, query)
	return topMinerInfo, err
}

//AddNodeInfo add node info into database
func (c *Client) AddNodeInfo(nodeInfo *DBNodeInfo) error {
	query := func(c *mgo.Collection) error {
		return c.Insert(nodeInfo)
	}
	err := c.withCollection(nodeInfoTbl, query)
	return err
}

//DeleteNodeInfo delete node info from database
func (c *Client) DeleteNodeInfo(nodeInfo *DBNodeInfo) error {
	query := func(c *mgo.Collection) error {
		return c.Remove(bson.M{"id": nodeInfo.ID})
	}
	err := c.withCollection(nodeInfoTbl, query)
	return err
}

//GetNodeInfo get node info from database
func (c *Client) GetNodeInfo(host string) (*DBNodeInfo, error) {
	dbNodeInfo := new(DBNodeInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"host": host}).One(dbNodeInfo)
	}
	err := c.withCollection(nodeInfoTbl, query)
	return dbNodeInfo, err
}

//GetNodeInfoByID get node info from database by node id
func (c *Client) GetNodeInfoByID(id string) (*DBNodeInfo, error) {
	dbNodeInfo := new(DBNodeInfo)
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"id": id}).One(dbNodeInfo)
	}
	err := c.withCollection(nodeInfoTbl, query)
	return dbNodeInfo, err
}

//GetNodeInfosByShardNumber get all node infos from database by shardNumber
func (c *Client) GetNodeInfosByShardNumber(shardNumber int) ([]*DBNodeInfo, error) {
	var nodeInfos []*DBNodeInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{"shardNumber": shardNumber}).All(&nodeInfos)
	}
	err := c.withCollection(nodeInfoTbl, query)
	return nodeInfos, err
}

//GetNodeInfos get all node infos from database
func (c *Client) GetNodeInfos() ([]*DBNodeInfo, error) {
	var nodeInfos []*DBNodeInfo
	query := func(c *mgo.Collection) error {
		return c.Find(bson.M{}).All(&nodeInfos)
	}
	err := c.withCollection(nodeInfoTbl, query)
	return nodeInfos, err
}

//GetNodeCntByShardNumber get row count of the node table
func (c *Client) GetNodeCntByShardNumber(shardNumber int) (uint64, error) {
	var NodeCnt uint64
	query := func(c *mgo.Collection) error {
		var err error
		//TODO: fix this overflow
		var temp int
		temp, err = c.Find(bson.M{"shardNumber": shardNumber}).Count()
		NodeCnt = uint64(temp)
		return err
	}
	err := c.withCollection(nodeInfoTbl, query)
	return NodeCnt, err
}
