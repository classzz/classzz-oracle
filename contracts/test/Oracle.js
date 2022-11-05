const {
  time,
  loadFixture,
} = require("@nomicfoundation/hardhat-network-helpers");
const { anyValue } = require("@nomicfoundation/hardhat-chai-matchers/withArgs");
const { expect } = require("chai");

describe("Oracle", function () {

  async function deployOneYearLockFixture() {

    const OffchainAggregator = await ethers.getContractFactory("OffchainAggregator");
    const offchainAggregator = await OffchainAggregator.deploy(100000, 100000, 18, 'demo');

    const AggregatorProxy = await ethers.getContractFactory("AggregatorProxy");
    const aggregatorProxy = await AggregatorProxy.deploy(offchainAggregator.address);

    return { offchainAggregator, aggregatorProxy};
  }

  it("test1", async function () {

    const { offchainAggregator, aggregatorProxy } = await loadFixture(deployOneYearLockFixture);
    // Contracts are deployed using the first signer/account by default

    const version = offchainAggregator.address;
    console.log(version);

    const times = await aggregatorProxy.latestTimestamp()
    console.log(times)

  });
});
