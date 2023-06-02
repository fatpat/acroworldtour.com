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
  const { name, results } = competition;

  const overallResults = results.overall_results;
  overallResults.sort((a, b) => b.score - a.score);

  const runsResults = results.runs_results;

  const [showOverall, setShowOverall] = useState(false);
  const [showRuns, setShowRuns] = useState(runsResults.map(() => false));

  const [hideExtra, setHideExtra] = useState(false);

  useEffect(() => {
    const shouldHide = showOverall || showRuns.some((showRun) => showRun);
    setHideExtra(shouldHide);
  }, [showOverall, showRuns]);

  const toggleResults = (index: "overall" | number) => {
    const newShowRuns = [...showRuns].fill(false);
    if (index !== "overall") newShowRuns[index] = !showRuns[index];

    setShowOverall(index === "overall" ? !showOverall : false);
    setShowRuns(newShowRuns);
  };

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
            hideExtra && "landscape:hidden"
          )}
        />

        {overallResults.length > 0 && (
          <section
            className={classNames(
              "flex w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner",
              hideExtra ? "lg:col-span-full" : "lg:col-span-6 lg:col-start-4"
            )}
          >
            {overallResults.length > 0 && (
              <button
                title="Click to open/close overall results"
                className={classNames(
                  "col-span-full flex cursor-pointer items-baseline justify-center"
                )}
                onClick={() => toggleResults("overall")}
                onKeyDown={({ key }) =>
                  key === "Enter" && toggleResults("overall")
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
                    onClick={() => toggleResults(runIndex)}
                    onKeyDown={({ key }) =>
                      key === "Enter" && toggleResults(runIndex)
                    }
                  >
                    <h3>{`Run ${runNumber}`}</h3>
                    <ChevronIcon
                      className={classNames(
                        "ml-2 h-3 w-auto",
                        !showRuns[runIndex] && "-rotate-90"
                      )}
                    />
                  </button>
                  {showRuns[runIndex] && (
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
