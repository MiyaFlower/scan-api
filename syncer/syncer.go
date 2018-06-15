package syncer

import (
	"fmt"

	"github.com/seeleteam/scan-api/database"
	"github.com/seeleteam/scan-api/log"
	"github.com/seeleteam/scan-api/rpc"

	"time"
)

//Syncer
type Syncer struct {
	rpc         *rpc.SeeleRPC
	db          Database
	shardNumber int
	syncCnt     int
}

//NewSyncer return a syncer to sync block data from seele node
func NewSyncer(db Database, rpcConnUrl string, shardNumber int) *Syncer {
	rpc := rpc.NewRPC(rpcConnUrl)
	if rpc == nil {
		return nil
	}

	if err := rpc.Connect(); err != nil {
		fmt.Printf("rpc init failed, connurl:%v\n", rpcConnUrl)
		return nil
	}

	return &Syncer{
		db:          db,
		rpc:         rpc,
		shardNumber: shardNumber,
		syncCnt:     0,
	}
}

//Blocks that are already in storage may be modified
func (s *Syncer) checkOlderBlocks() bool {
	dbBlockHeight, err := s.db.GetBlockHeight(s.shardNumber)
	if err != nil {
		log.Error(err)
		return false
	}

	if dbBlockHeight == 0 {
		return false
	}

	fallBack := false
	for i := dbBlockHeight - 1; i >= 0; i-- {
		rpcBlock, err := s.rpc.GetBlockByHeight(i, true)
		if err != nil {
			return fallBack
		}

		dbBlock, err := s.db.GetBlockByHeight(s.shardNumber, i)
		if err != nil {
			return fallBack
		}

		//if the block data is different wo should sync the data again
		if dbBlock.HeadHash == rpcBlock.Hash {
			return fallBack
		}

		//Delete dbBlock
		s.db.RemoveBlock(i)

		//Delete txs
		s.db.RemoveTxs(i)

		//Modify accounts
		for j := 0; j < len(dbBlock.Txs); j++ {
			tx := dbBlock.Txs[j]

			toAccount, err := s.db.GetAccountByAddress(tx.To)
			if err != nil {
				log.Error(err)
				return fallBack
			}

			toAccount.Balance, err = s.rpc.GetBalance(tx.To)
			if err != nil {
				log.Error(err)
				toAccount.Balance = 0
			}

			txCnt, err := s.db.GetTxCntByShardNumberAndAddress(s.shardNumber, tx.From)
			if err != nil {
				log.Error(err)
				txCnt = 0
			}

			s.db.UpdateAccount(tx.To, toAccount.Balance, txCnt)

			if tx.From != nullAddress {
				fromAccount, err := s.db.GetAccountByAddress(tx.From)

				fromAccount.Balance, err = s.rpc.GetBalance(tx.From)
				if err != nil {
					log.Error(err)
					fromAccount.Balance = 0
				}

				txCnt, err := s.db.GetTxCntByShardNumberAndAddress(s.shardNumber, tx.From)
				if err != nil {
					log.Error(err)
					txCnt = 0
				}

				s.db.UpdateAccount(tx.To, fromAccount.Balance, txCnt)
			}
		}

		if dbBlock.Creator != nullAddress {
			minerAccount, err := s.db.GetAccountByAddress(dbBlock.Creator)
			if err != nil {
				minerAccount = database.CreateEmptyAccount(dbBlock.Creator, s.shardNumber)
				err := s.db.AddAccount(minerAccount)
				if err != nil {
					log.Error("[DB] err : %v", err)
				} else {
					minerAccount, err = s.db.GetAccountByAddress(dbBlock.Creator)
					if err != nil {
						log.Error("[DB] err : %v", err)
					}
				}
			}

			blockCnt, err := s.db.GetMinedBlocksCntByShardNumberAndAddress(s.shardNumber, dbBlock.Creator)
			if err != nil {
				log.Error(err)
				blockCnt = 0
			}

			s.db.UpdateAccountMinedBlock(dbBlock.Creator, blockCnt)
		}

		fallBack = true
	}

	return fallBack
}

//sync get block data from seele node and store it in the mongodb
func (s *Syncer) sync() error {
	log.Info("[BlockSync syncCnt:%d]Begin Sync", s.syncCnt)

	s.checkOlderBlocks()

	curBlock, err := s.rpc.CurrentBlock()
	if err != nil {
		log.Error(err)
		return err
	}

	dbBlockHeight, err := s.db.GetBlockHeight(s.shardNumber)
	if err != nil {
		log.Error(err)
		return err
	}

	for i := dbBlockHeight; i <= curBlock.Height; i++ {
		rpcBlock, err := s.rpc.GetBlockByHeight(i, true)
		if err != nil {
			s.rpc.Release()
			log.Error(err)
			break
		}

		err = s.blockSync(rpcBlock)
		if err != nil {
			log.Error(err)
			break
		}

		err = s.txSync(rpcBlock)
		if err != nil {
			log.Error(err)
			break
		}

		err = s.accountSync(rpcBlock)
		if err != nil {
			log.Error(err)
			break
		}
	}

	err = s.pendingTxsSync()
	if err != nil {
		log.Error(err)
	}
	log.Info("[BlockSync syncCnt:%d]End Sync", s.syncCnt)
	s.syncCnt++
	return nil
}

//StartSync start an timer to sync block data from seele node
func (s *Syncer) StartSync(interval time.Duration) {
	s.sync()

	ticks := time.NewTicker(interval * time.Second)
	tick := ticks.C
	go func() {
		for range tick {
			s.sync()
			_, ok := <-tick
			if !ok {
				break
			}
		}
	}()
}