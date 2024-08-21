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
  let globalResults: components["schemas"]["CompetitionResultsExport"]["results"] =
    results.results;

  for (var resultsType in globalResults) {
    globalResults[resultsType]?.sort((a, b) => b.score - a.score);
  }
  const runsResults = results.runs_results;

  const [hideSummary, setHideSummary] = useState(false);

  let defaultShowResults: { [index: string]: any } = {};
  Object.keys(globalResults).map(
    (t) => (defaultShowResults[`overall-${t}`] = false),
  );
  runsResults.map((r, i) =>
    Object.keys(r.results).map(
      (t) => (defaultShowResults[`run${i}-${t}`] = false),
    ),
  );
  const [showResults, setShowResults] = useState(
    JSON.parse(
      localStorage.getItem(`competitions/${code}/showResults`) || "false",
    ) || defaultShowResults,
  );

  const changeResults = (index: string) => {
    const newShowResults = Object.fromEntries(
      Object.values(showResults).map((k) => [k, false]),
    );
    newShowResults[index] = !showResults[index];
    setShowResults(newShowResults);
  };

  useEffect(() => {
    localStorage.setItem(
      `competitions/${code}/showResults`,
      JSON.stringify(showResults),
    );

    setHideSummary(Object.values(showResults).some((b) => b));
  }, [showResults, code]);

  const compText = competition.results.final ? "" : " (on-going)";
  return (
    <>
      <h2 className="mb-4">{name}</h2>
      {(Object.keys(globalResults).length === 0 ||
        !Object.values(globalResults).some(
          (r) => r instanceof Array && r.length > 0,
        )) && <h3>No results at the moment.</h3>}
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
          Object.values(globalResults).some(
            (r) => r instanceof Array && r.length > 0,
          ) && (
            <>
              <section
                key="overall"
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

                <h3 className="capitalize">Overall Results{compText}</h3>
                {Object.keys(globalResults).map((resultsType) => {
                  if (
                    resultsType == "overall" &&
                    Object.keys(globalResults).length > 1
                  )
                    return;
                  return (
                    <Fragment key={`overall-${resultsType}`}>
                      <button
                        title={`Click to open/close ${resultsType} results`}
                        className={classNames(
                          "col-span-full flex cursor-pointer items-baseline justify-center",
                        )}
                        onClick={() => changeResults(`overall-${resultsType}`)}
                        onKeyDown={({ key }) =>
                          key === "Enter" &&
                          changeResults(`overall-${resultsType}`)
                        }
                      >
                        <h4 className="capitalize">
                          {resultsType === "overall" ? "open" : resultsType}
                        </h4>
                        <ChevronIcon
                          className={classNames(
                            "ml-2 h-3 w-auto",
                            !showResults[`overall-${resultsType}`] &&
                              "-rotate-90",
                          )}
                        />
                      </button>
                      {showResults[`overall-${resultsType}`] && (
                        <CompetitionOverallResults
                          results={globalResults[resultsType]}
                          type={type}
                          className="grid grid-cols-12"
                        />
                      )}
                    </Fragment>
                  );
                })}
              </section>

              {runsResults.map((run, runIndex) => {
                const runNumber = runIndex + 1;
                const runText = run.final ? "" : " (provisional)";
                return (
                  <>
                    <section
                      key={`run${runIndex}`}
                      className={classNames(
                        "flex w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner",
                        hideSummary
                          ? "lg:col-span-full"
                          : "lg:col-span-6 lg:col-start-4",
                      )}
                    >
                      <h3>
                        run {runNumber}
                        {runText}
                      </h3>
                      {Object.keys(run.results).map((resultsType) => {
                        if (
                          resultsType == "overall" &&
                          Object.keys(run.results).length > 1
                        )
                          return;
                        return (
                          <Fragment key={`run${runIndex}-${resultsType}`}>
                            <button
                              title="Click to open/close run results"
                              className={classNames(
                                "col-span-full flex cursor-pointer items-baseline justify-center",
                              )}
                              onClick={() =>
                                changeResults(`run${runIndex}-${resultsType}`)
                              }
                              onKeyDown={({ key }) =>
                                key === "Enter" &&
                                changeResults(`run${runIndex}-${resultsType}`)
                              }
                            >
                              <h4 className="capitalize">
                                {resultsType === "overall"
                                  ? "open"
                                  : resultsType}
                              </h4>
                              <ChevronIcon
                                className={classNames(
                                  "ml-2 h-3 w-auto",
                                  !showResults[
                                    `run${runIndex}-${resultsType}`
                                  ] && "-rotate-90",
                                )}
                              />
                            </button>
                            {showResults[`run${runIndex}-${resultsType}`] && (
                              <CompetitionRunMain
                                results={run.results[resultsType]}
                                type={run.type as CompetitionType}
                                className="grid grid-cols-12 border-[1px]"
                              />
                            )}
                          </Fragment>
                        );
                      })}
                    </section>
                  </>
                );
              })}
            </>
          )}
      </div>
    </>
  );
};

export default CompetitionDetails;
