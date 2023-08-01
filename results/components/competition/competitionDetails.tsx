import classNames from "classnames";
import { Fragment, useEffect, useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";
import CompetitionOverallResults from "./competitionOverallResults";
import CompetitionRunMain from "./competitionRunMain";
import CompetitionSummary from "./competitionSummary";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

type CompetitionType = components["schemas"]["CompetitionType"];

const CompetitionDetails = ({ competition }: Props) => {
  const { name, results, code, type } = competition;
  const globalResults = results.results;
  for (var resultsType in globalResults) {
    globalResults[resultsType].sort((a, b) => b.score - a.score);
  }
  const runsResults = results.runs_results;

  const [hideSummary, setHideSummary] = useState(false);
  const [showGlobal, setShowGlobal] = useState(
    JSON.parse(
      localStorage.getItem(`competitions/${code}/showGlobal`) || "false",
    ) || Object.fromEntries(Object.keys(globalResults).map((k) => [k, false])),
  );
  const [showRun, setShowRun] = useState(
    JSON.parse(
      localStorage.getItem(`competitions/${code}/showRun`) || "false",
    ) || runsResults.map(() => false),
  );

  const changeResults = (index: str | number) => {
    const newShowGlobal = Object.fromEntries(
      Object.values(showGlobal).map((k) => [k, false]),
    );
    const newShowRuns = [...showRun].fill(false);
    if (typeof index === "number") newShowRuns[index] = !showRun[index];
    if (typeof index === "string") newShowGlobal[index] = !showGlobal[index];

    setShowGlobal(newShowGlobal);
    setShowRun(newShowRuns);
  };

  useEffect(() => {
    localStorage.setItem(
      `competitions/${code}/showGlobal`,
      JSON.stringify(showGlobal),
    );
    localStorage.setItem(
      `competitions/${code}/showRun`,
      JSON.stringify(showRun),
    );

    setHideSummary(
      Object.values(showGlobal).some((showGlobal: boolean) => showGlobal) ||
        showRun.some((showRun: boolean) => showRun),
    );
  }, [showGlobal, showRun, code]);

  const compText = competition.results.final ? "" : " (on-going)";
  return (
    <>
      <h2 className="mb-4">{name}</h2>
      {(Object.keys(globalResults).length === 0 ||
        !Object.values(globalResults).some((r) => r.length > 0)) && (
        <h3>No results at the moment.</h3>
      )}
      <div
        className={classNames(
          "mt-4 flex w-full items-start justify-center gap-4 portrait:flex-col",
        )}
      >
        <CompetitionSummary
          competition={competition}
          className={classNames(
            "w-1/2 max-w-lg rounded-xl bg-awt-dark-50 px-2 py-2 pb-2 shadow-inner",
            "portrait:w-full",
            hideSummary && "landscape:hidden",
          )}
        />

        {Object.keys(globalResults).length > 0 &&
          Object.values(globalResults).some((r) => r.length > 0) && (
            <section
              className={classNames(
                "flex w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner",
                hideSummary
                  ? "lg:col-span-full"
                  : "lg:col-span-6 lg:col-start-4",
              )}
            >
              {hideSummary && (
                <small className="text-center">
                  Results are updated automatically.
                </small>
              )}

              {Object.keys(globalResults).map((resultsType) => {
                return (
                  <Fragment key={resultsType}>
                    <button
                      title="Click to open/close ${resultsType} results"
                      className={classNames(
                        "col-span-full flex cursor-pointer items-baseline justify-center",
                      )}
                      onClick={() => changeResults(resultsType)}
                      onKeyDown={({ key }) =>
                        key === "Enter" && changeResults(resultsType)
                      }
                    >
                      <h3 className="capitalize">
                        {resultsType}
                        {compText}
                      </h3>
                      <ChevronIcon
                        className={classNames(
                          "ml-2 h-3 w-auto",
                          !showGlobal[resultsType] && "-rotate-90",
                        )}
                      />
                    </button>
                    {showGlobal[resultsType] && (
                      <CompetitionOverallResults
                        results={globalResults[resultsType]}
                        type={type}
                        className="grid grid-cols-12"
                      />
                    )}
                  </Fragment>
                );
              })}

              {runsResults.map((run, runIndex) => {
                const runNumber = runIndex + 1;
                const runText = run.final ? "" : " (on-going)";
                if (run.results["overall"].length > 0)
                  return (
                    <Fragment key={runIndex}>
                      <button
                        title="Click to open/close run results"
                        className={classNames(
                          "col-span-full flex cursor-pointer items-baseline justify-center",
                        )}
                        onClick={() => changeResults(runIndex)}
                        onKeyDown={({ key }) =>
                          key === "Enter" && changeResults(runIndex)
                        }
                      >
                        <h3 className="capitalize">{`Run ${runNumber}${runText}`}</h3>
                        <ChevronIcon
                          className={classNames(
                            "ml-2 h-3 w-auto",
                            !showRun[runIndex] && "-rotate-90",
                          )}
                        />
                      </button>
                      {showRun[runIndex] && (
                        <CompetitionRunMain
                          results={run.results["overall"]}
                          type={run.type as CompetitionType}
                          className="grid grid-cols-12 border-[1px]"
                        />
                      )}
                    </Fragment>
                  );
              })}
            </section>
          )}
      </div>
    </>
  );
};

export default CompetitionDetails;
