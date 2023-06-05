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

const CompetitionDetails = ({ competition }: Props) => {
  const { name, results, code } = competition;
  const overallResults = results.overall_results;
  overallResults.sort((a, b) => b.score - a.score);
  const runsResults = results.runs_results;

  const [hideSummary, setHideSummary] = useState(false);
  const [showOverall, setShowOverall] = useState(
    JSON.parse(localStorage.getItem(`competitions/${code}/showOverall`) || "false") || false
  );
  const [showRun, setShowRun] = useState(
    JSON.parse(localStorage.getItem(`competitions/${code}/showRun`) || "false") ||
      runsResults.map(() => false)
  );

  const changeResults = (index: "overall" | number) => {
    const newShowRuns = [...showRun].fill(false);
    if (index !== "overall") newShowRuns[index] = !showRun[index];

    setShowOverall(index === "overall" ? !showOverall : false);
    setShowRun(newShowRuns);
  };

  useEffect(() => {
    localStorage.setItem(`competitions/${code}/showOverall`, JSON.stringify(showOverall));
    localStorage.setItem(`competitions/${code}/showRun`, JSON.stringify(showRun));

    setHideSummary(showOverall || showRun.some((showRun: boolean) => showRun));
  }, [showOverall, showRun, code]);

  return (
    <>
      <h2 className="mb-4">{name}</h2>
      {overallResults.length === 0 && <h3>No results at the moment.</h3>}
      <div
        className={classNames(
          "mt-4 flex w-full items-start justify-center gap-4 portrait:flex-col"
        )}
      >
        <CompetitionSummary
          competition={competition}
          className={classNames(
            "w-1/2 max-w-lg rounded-xl bg-awt-dark-50 px-2 py-2 pb-2 shadow-inner",
            "portrait:w-full",
            hideSummary && "landscape:hidden"
          )}
        />

        {overallResults.length > 0 && (
          <section
            className={classNames(
              "flex w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner",
              hideSummary ? "lg:col-span-full" : "lg:col-span-6 lg:col-start-4"
            )}
          >
            {hideSummary && (
              <small className="text-center">
                Results are updated automatically.
              </small>
            )}

            {overallResults.length > 0 && (
              <button
                title="Click to open/close overall results"
                className={classNames(
                  "col-span-full flex cursor-pointer items-baseline justify-center"
                )}
                onClick={() => changeResults("overall")}
                onKeyDown={({ key }) =>
                  key === "Enter" && changeResults("overall")
                }
              >
                <h3>Overall Results</h3>
                <ChevronIcon
                  className={classNames(
                    "ml-2 h-3 w-auto",
                    !showOverall && "-rotate-90"
                  )}
                />
              </button>
            )}

            {showOverall && (
              <CompetitionOverallResults
                results={overallResults}
                className="grid grid-cols-12"
              />
            )}

            {runsResults.map((run, runIndex) => {
              const runNumber = runIndex + 1;

              return (
                <Fragment key={runIndex}>
                  <button
                    title="Click to open/close run results"
                    className={classNames(
                      "col-span-full flex cursor-pointer items-baseline justify-center"
                    )}
                    onClick={() => changeResults(runIndex)}
                    onKeyDown={({ key }) =>
                      key === "Enter" && changeResults(runIndex)
                    }
                  >
                    <h3>{`Run ${runNumber}`}</h3>
                    <ChevronIcon
                      className={classNames(
                        "ml-2 h-3 w-auto",
                        !showRun[runIndex] && "-rotate-90"
                      )}
                    />
                  </button>
                  {showRun[runIndex] && (
                    <CompetitionRunMain
                      results={run.results}
                      type={run.type}
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
