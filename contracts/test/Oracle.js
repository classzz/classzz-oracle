const {
  time,
  loadFixture,
} = require("@nomicfoundation/hardhat-network-helpers");
const { anyValue } = require("@nomicfoundation/hardhat-chai-matchers/withArgs");
const { expect } = require("chai");

describe("Oracle", function () {

  async function deployOneYearLockFixture() {

    const OffchainAggregator = await ethers.getContractFactory("OffchainAggregator");
    const offchainAggregator = await OffchainAggregator.deploy(ethers.utils.parseEther("0.00000000001"), ethers.utils.parseEther("0.000001"), 8, 'demo');

    const AggregatorProxy = await ethers.getContractFactory("AggregatorProxy");
    const aggregatorProxy = await AggregatorProxy.deploy(offchainAggregator.address);

    return { offchainAggregator, aggregatorProxy};
  }

  it("test1", async function () {

    const { offchainAggregator, aggregatorProxy } = await loadFixture(deployOneYearLockFixture);

    // Contracts are deployed using the first signer/account by default
    const accounts = await ethers.getSigners();

    const signers = [accounts[1].address, accounts[2].address, accounts[3].address, accounts[4].address, accounts[5].address]
    console.log(signers)
    await offchainAggregator.setSigners(signers)

    const roundId = 1;
    const answer = ethers.utils.parseEther("0.0000001");
    const answer1 = ethers.utils.parseEther("0.00000011");
    const answer2 = ethers.utils.parseEther("0.00000012");
    console.log(answer, answer1, answer2)

    await offchainAggregator.connect(accounts[1]).transmit(roundId, answer)
    await offchainAggregator.connect(accounts[2]).transmit(roundId, answer1)
    await offchainAggregator.connect(accounts[3]).transmit(roundId, answer2)

    const data1 = await aggregatorProxy.latestRoundData()
    console.log(data1)

  });
});
