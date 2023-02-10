// SPDX-License-Identifier: MIT
pragma solidity 0.8.10;
import "@openzeppelin/contracts/access/Ownable.sol";

import "./AggregatorV2V3Interface.sol";
import "./TypeAndVersionInterface.sol";
import 'hardhat/console.sol';

/**
  * @notice Onchain verification of reports from the offchain reporting protocol

  * @dev For details on its operation, see the offchain reporting protocol design
  * @dev doc, which refers to this contract as simply the "contract".
*/
contract OffchainAggregator is Ownable, AggregatorV2V3Interface, TypeAndVersionInterface {

  uint256 constant internal maxNumOracles = 31;

  // Transmission records the median answer from the transmit transaction at
  // time timestamp
  struct Transmission {
    int192 answer; // 192 bits ought to be enough for anyone
    uint64 timestamp;
  }

  mapping(uint32 /* aggregator round ID */ => Transmission) internal s_transmissions;
  mapping(uint32 /* aggregator round ID */ => mapping(address => Transmission)) internal s_signers_transmissions;

  // into the config digest, to prevent replay attacks.
  uint32 internal s_latestConfigBlockNumber; // makes it easier for offchain systems

  uint32 internal s_latestAggregatorRoundId;

  // Lowest answer the system is allowed to report in response to transmissions
  int192 immutable public minAnswer;
  // Highest answer the system is allowed to report in response to transmissions
  int192 immutable public maxAnswer;

  // s_signers contains the signing address of each oracle
  address[] internal s_signers;

  /*
   * @param _minAnswer lowest answer the median of a report is allowed to be
   * @param _maxAnswer highest answer the median of a report is allowed to be
   * @param _decimals answers are stored in fixed-point format, with this many digits of precision
   * @param _description short human-readable description of observable this contract's answers pertain to
   */
  constructor(
    int192 _minAnswer,
    int192 _maxAnswer,
    uint8 _decimals,
    string memory _description
  )
  {
    decimals = _decimals;
    s_description = _description;
    minAnswer = _minAnswer;
    maxAnswer = _maxAnswer;
  }

  /*
   * Versioning
   */
  function typeAndVersion()
  external
  override
  pure
  virtual
  returns (string memory)
  {
    return "OffchainAggregator 2.0.0";
  }

  /**
   * @notice triggers a new run of the offchain reporting protocol
   * @param previousConfigBlockNumber block in which the previous config was set, to simplify historic analysis
   * @param signers ith element is address ith oracle uses to sign a report
   */
  event ConfigSet(
    uint32 previousConfigBlockNumber,
    address[] signers
  );

  /**
   * @notice sets offchain reporting protocol configuration incl. participating oracles
   * @param _signers addresses with which oracles sign the reports
   */
  function setSigners(
    address[] calldata _signers
  )
  external
  onlyOwner()
  {

    require(_signers.length <= maxNumOracles, "too many signers");
    for (uint i = 0; i < s_signers.length; i++) { // add new signer/transmitter addresses
      s_signers.pop();
    }

    for (uint i = 0; i < _signers.length; i++) { // add new signer/transmitter addresses
      s_signers.push(_signers[i]);
    }

    uint32 previousConfigBlockNumber = s_latestConfigBlockNumber;
    s_latestConfigBlockNumber = uint32(block.number);

    emit ConfigSet(
      previousConfigBlockNumber,
      _signers
    );
  }

  /*
   * Transmission logic
   */

  /**
   * @notice indicates that a new report was transmitted
   * @param aggregatorRoundId the round to which this report was assigned
   * @param answer median of the observations attached this report
   */
  event NewTransmission(
    uint32 indexed aggregatorRoundId,
    int192 answer
  );

  function transmit(
    uint32 roundId,
    int192 answer
  )
  external
  {
    uint temp = 0;
    for (uint i = 0; i < s_signers.length; i++) { // add new signer/transmitter addresses
      if (s_signers[i] == msg.sender){
        temp = 1;
        break;
      }
    }

    require(temp != 0, "signer does not exist");
    require(s_signers_transmissions[roundId][msg.sender].answer == 0, "Admin repeated submission");
    require(roundId > s_latestAggregatorRoundId, "roundId > s_latestAggregatorRoundId");

    s_signers_transmissions[roundId][msg.sender] = Transmission(answer, uint64(block.timestamp));

    require(minAnswer <= answer && answer <= maxAnswer, "median is out of min-max range");
    s_transmissions[roundId] = Transmission(answer, uint64(block.timestamp));
    s_latestAggregatorRoundId++;

    emit NewTransmission(
      s_latestAggregatorRoundId,
      answer
    );

    // Emit these for backwards compatability with offchain consumers
    // that only support legacy events
    emit NewRound(
      s_latestAggregatorRoundId,
      address(0x0), // use zero address since we don't have anybody "starting" the round here
      block.timestamp
    );

    emit AnswerUpdated(
      answer,
      s_latestAggregatorRoundId,
      block.timestamp
    );
    
  }

  /*
   * v2 Aggregator interface
   */

  /**
   * @notice median from the most recent report
   */
  function latestAnswer()
  public
  override
  view
  virtual
  returns (int256)
  {
    return s_transmissions[s_latestAggregatorRoundId].answer;
  }

  /**
   * @notice timestamp of block in which last report was transmitted
   */
  function latestTimestamp()
  public
  override
  view
  virtual
  returns (uint256)
  {
    return s_transmissions[s_latestAggregatorRoundId].timestamp;
  }

  /**
   * @notice Aggregator round (NOT OCR round) in which last report was transmitted
   */
  function latestRound()
  public
  override
  view
  virtual
  returns (uint256)
  {
    return s_latestAggregatorRoundId;
  }

  /**
   * @notice median of report from given aggregator round (NOT OCR round)
   * @param _roundId the aggregator round of the target report
   */
  function getAnswer(uint256 _roundId)
  public
  override
  view
  virtual
  returns (int256)
  {
    if (_roundId > 0xFFFFFFFF) { return 0; }
    return s_transmissions[uint32(_roundId)].answer;
  }

  /**
   * @notice timestamp of block in which report from given aggregator round was transmitted
   * @param _roundId aggregator round (NOT OCR round) of target report
   */
  function getTimestamp(uint256 _roundId)
  public
  override
  view
  virtual
  returns (uint256)
  {
    if (_roundId > 0xFFFFFFFF) { return 0; }
    return s_transmissions[uint32(_roundId)].timestamp;
  }

  /*
   * v3 Aggregator interface
   */

  string constant private V3_NO_DATA_ERROR = "No data present";

  /**
   * @return answers are stored in fixed-point format, with this many digits of precision
   */
  uint8 immutable public override decimals;

  /**
   * @notice aggregator contract version
   */
  uint256 constant public override version = 4;

  string internal s_description;

  /**
   * @notice human-readable description of observable this contract is reporting on
   */
  function description()
  public
  override
  view
  virtual
  returns (string memory)
  {
    return s_description;
  }

  /**
   * @notice details for the given aggregator round
   * @param _roundId target aggregator round (NOT OCR round). Must fit in uint32
   * @return roundId _roundId
   * @return answer median of report from given _roundId
   * @return startedAt timestamp of block in which report from given _roundId was transmitted
   * @return updatedAt timestamp of block in which report from given _roundId was transmitted
   * @return answeredInRound _roundId
   */
  function getRoundData(uint80 _roundId)
  public
  override
  view
  virtual
  returns (
    uint80 roundId,
    int256 answer,
    uint256 startedAt,
    uint256 updatedAt,
    uint80 answeredInRound
  )
  {
    require(_roundId <= 0xFFFFFFFF, V3_NO_DATA_ERROR);
    Transmission memory transmission = s_transmissions[uint32(_roundId)];
    return (
    _roundId,
    transmission.answer,
    transmission.timestamp,
    transmission.timestamp,
    _roundId
    );
  }

  /**
   * @notice aggregator details for the most recently transmitted report
   * @return roundId aggregator round of latest report (NOT OCR round)
   * @return answer median of latest report
   * @return startedAt timestamp of block containing latest report
   * @return updatedAt timestamp of block containing latest report
   * @return answeredInRound aggregator round of latest report
   */
  function latestRoundData()
  public
  override
  view
  virtual
  returns (
    uint80 roundId,
    int256 answer,
    uint256 startedAt,
    uint256 updatedAt,
    uint80 answeredInRound
  )
  {
    roundId = s_latestAggregatorRoundId;

    // Skipped for compatability with existing FluxAggregator in which latestRoundData never reverts.
    Transmission memory transmission = s_transmissions[uint32(roundId)];
    return (
    roundId,
    transmission.answer,
    transmission.timestamp,
    transmission.timestamp,
    roundId
    );
  }
}