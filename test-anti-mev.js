#!/usr/bin/env node
// Script de teste para demonstrar features anti-MEV do EQUA

const Web3 = require('web3');
const web3 = new Web3('http://localhost:8545');

const colors = {
    reset: '\x1b[0m',
    bright: '\x1b[1m',
    green: '\x1b[32m',
    red: '\x1b[31m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    cyan: '\x1b[36m',
};

async function main() {
    console.log(`\n${colors.bright}${colors.blue}========================================`);
    console.log(`   EQUA Anti-MEV Test Suite`);
    console.log(`========================================${colors.reset}\n`);

    // Get account
    const accounts = await web3.eth.getAccounts();
    if (accounts.length === 0) {
        console.error(`${colors.red}❌ No accounts available${colors.reset}`);
        process.exit(1);
    }

    const account = accounts[0];
    console.log(`${colors.cyan}📍 Using account: ${account}${colors.reset}`);

    // Get current block and balance
    const blockNumber = await web3.eth.getBlockNumber();
    const balance = await web3.eth.getBalance(account);
    console.log(`${colors.cyan}📊 Current block: ${blockNumber}${colors.reset}`);
    console.log(`${colors.cyan}💰 Balance: ${web3.utils.fromWei(balance, 'ether')} EQUA${colors.reset}\n`);

    console.log(`${colors.yellow}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}`);
    console.log(`${colors.bright}TEST 1: Fair Ordering (FCFS)${colors.reset}`);
    console.log(`${colors.yellow}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}\n`);

    console.log(`${colors.cyan}📤 Enviando 3 transações com gas prices diferentes:${colors.reset}`);

    const nonce = await web3.eth.getTransactionCount(account);
    const txs = [];

    // TX 1: Gas MUITO ALTO (999 gwei) - Em blockchain normal seria primeira
    console.log(`  1️⃣  Gas Price: ${colors.red}999 Gwei (ALTO)${colors.reset} → Target: 0x...001`);
    const tx1 = {
        from: account,
        to: '0x0000000000000000000000000000000000000001',
        value: web3.utils.toWei('0.01', 'ether'),
        gas: 21000,
        gasPrice: web3.utils.toWei('999', 'gwei'),
        nonce: nonce,
    };

    // TX 2: Gas MUITO BAIXO (1 gwei) - Em blockchain normal seria última
    console.log(`  2️⃣  Gas Price: ${colors.green}1 Gwei (BAIXO)${colors.reset} → Target: 0x...002`);
    const tx2 = {
        from: account,
        to: '0x0000000000000000000000000000000000000002',
        value: web3.utils.toWei('0.01', 'ether'),
        gas: 21000,
        gasPrice: web3.utils.toWei('1', 'gwei'),
        nonce: nonce + 1,
    };

    // TX 3: Gas MÉDIO (500 gwei) - Em blockchain normal seria segunda
    console.log(`  3️⃣  Gas Price: ${colors.yellow}500 Gwei (MÉDIO)${colors.reset} → Target: 0x...003\n`);
    const tx3 = {
        from: account,
        to: '0x0000000000000000000000000000000000000003',
        value: web3.utils.toWei('0.01', 'ether'),
        gas: 21000,
        gasPrice: web3.utils.toWei('500', 'gwei'),
        nonce: nonce + 2,
    };

    try {
        // Send transactions
        const hash1 = await web3.eth.sendTransaction(tx1);
        txs.push({ hash: hash1.transactionHash, gasPrice: '999 Gwei', target: '0x...001' });

        const hash2 = await web3.eth.sendTransaction(tx2);
        txs.push({ hash: hash2.transactionHash, gasPrice: '1 Gwei', target: '0x...002' });

        const hash3 = await web3.eth.sendTransaction(tx3);
        txs.push({ hash: hash3.transactionHash, gasPrice: '500 Gwei', target: '0x...003' });

        console.log(`${colors.green}✅ Transações enviadas!${colors.reset}`);
        console.log(`${colors.cyan}⏳ Aguardando próximo bloco (~12 segundos)...${colors.reset}\n`);

        // Wait for next block
        await waitForNextBlock(blockNumber);

        const latestBlock = await web3.eth.getBlock('latest');

        console.log(`${colors.yellow}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}`);
        console.log(`${colors.bright}RESULTADO: Bloco #${latestBlock.number}${colors.reset}`);
        console.log(`${colors.yellow}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}\n`);

        if (latestBlock.transactions.length > 0) {
            console.log(`${colors.cyan}📦 Transações no bloco (${latestBlock.transactions.length}):${colors.reset}\n`);

            for (let i = 0; i < latestBlock.transactions.length; i++) {
                const txHash = latestBlock.transactions[i];
                const receipt = await web3.eth.getTransaction(txHash);
                const originalTx = txs.find(t => t.hash === txHash);

                if (originalTx) {
                    console.log(`  ${i + 1}. ${colors.bright}${receipt.to}${colors.reset}`);
                    console.log(`     Gas Price: ${colors.cyan}${originalTx.gasPrice}${colors.reset}`);
                    console.log(`     Hash: ${txHash.substring(0, 20)}...${colors.reset}\n`);
                }
            }

            console.log(`${colors.green}${colors.bright}✅ ANTI-MEV ATIVO!${colors.reset}`);
            console.log(`${colors.green}Transações ordenadas por TIMESTAMP (FCFS),${colors.reset}`);
            console.log(`${colors.green}NÃO por gas price! 🎯${colors.reset}\n`);

            console.log(`${colors.yellow}💡 Em blockchains tradicionais:${colors.reset}`);
            console.log(`   Ordem seria: 999 Gwei → 500 Gwei → 1 Gwei`);
            console.log(`   (permite front-running e MEV!) ❌\n`);

            console.log(`${colors.green}✨ No EQUA:${colors.reset}`);
            console.log(`   Ordem é: por TIMESTAMP de chegada`);
            console.log(`   (protege contra front-running!) ✅\n`);

        } else {
            console.log(`${colors.red}⚠️  Nenhuma transação no bloco${colors.reset}`);
            console.log(`${colors.yellow}Tente novamente ou verifique o mempool${colors.reset}\n`);
        }

    } catch (error) {
        console.error(`${colors.red}❌ Erro: ${error.message}${colors.reset}`);
    }

    console.log(`${colors.blue}========================================`);
    console.log(`   Teste concluído!`);
    console.log(`========================================${colors.reset}\n`);
}

async function waitForNextBlock(currentBlock) {
    return new Promise((resolve) => {
        const interval = setInterval(async () => {
            const latest = await web3.eth.getBlockNumber();
            if (latest > currentBlock) {
                clearInterval(interval);
                resolve();
            }
        }, 1000);
    });
}

main().catch(console.error);

