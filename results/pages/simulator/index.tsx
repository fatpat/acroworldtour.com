import { useEffect, useState } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import SimulatorRun from "@/components/simulator/simulatorRun";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";

type UniqueTrick = components["schemas"]["UniqueTrick"];
type Flight = components["schemas"]["Flight"];

const Tricks = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Simulator");
    setPageDescription("Simulate your runs");

    setHeaderTitle("");
    setHeaderSubtitle("");
    setActiveNav("simulator");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const maxNumberOfRuns = 5;
  const defaultCompetitionName = "default";
  const defaultNumberOfRuns = 1;
  const defaultResetRepetitionsFrequency = 3;

  const [competitionName, setCompetitionName] = useState<string | undefined>(
    undefined,
  );
  const [numberOfRuns, setNumberOfRuns] = useState<number | undefined>(
    undefined,
  );
  const [resetRepetitionsFrequency, setResetRepetitionsFrequency] = useState<
    number | undefined
  >(undefined);

  const [runs, setRuns] = useState(Array(maxNumberOfRuns).fill(undefined));
  const [results, setResults] = useState(
    Array(maxNumberOfRuns).fill(undefined),
  );
  const [finalScore, setFinalScore] = useState<number | undefined>(undefined);

  const setRun = (i: number, run: any) => {
    if (i < 0 || i >= maxNumberOfRuns) return;
    let r = [...runs];
    r[i] = run;
    setRuns(r);
  };

  useEffect(() => {
    if (resetRepetitionsFrequency === undefined) return;

    fetch(
      `${API_URL}/simulate/competition/solo?reset_repetitions_frequency=${resetRepetitionsFrequency}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(
          runs.map((run) => {
            if (!run)
              return {
                tricks: [],
                marks: [
                  {
                    judge: "simulator",
                    technical: 0,
                    choreography: 0,
                    landing: 0,
                  },
                ],
                did_not_start: false,
                warnings: [],
              };

            return {
              tricks: run.tricks.filter((t: any) => t !== undefined),
              marks: [
                {
                  judge: "simulator",
                  technical: run.technical,
                  choreography: run.choreography,
                  landing: run.landing,
                },
              ],
              did_not_start: false,
              warnings: [],
            };
          }),
        ),
      },
    ).then(function (response) {
      if (!response.ok) {
        console.log(`received error, code=${response.status}`);
        return;
      }
      var contentType = response.headers.get("content-type");
      if (contentType && contentType.indexOf("application/json") === -1) {
        console.log("Error, content-type is not json");
        return;
      }
      return response.json().then(function (json) {
        let score = 0;
        setResults(
          json.map((r: Flight, i: number) => {
            if (!runs) return undefined;
            if (!runs[i]) return undefined;
            if (!runs[i].tricks) return undefined;
            if (!runs[i].tricks) return undefined;

            score += r.final_marks?.score || 0;

            return runs[i].tricks.filter((t: any) => t !== undefined).length > 0
              ? r
              : undefined;
          }),
        );
        setFinalScore(score);
      });
    });
  }, [runs, resetRepetitionsFrequency]);

  useEffect(() => {
    if (typeof window === "undefined" || !window.localStorage) return;

    if (competitionName === undefined) {
      let v = localStorage.getItem("simulator/competitionName");
      setCompetitionName(v === null ? defaultCompetitionName : JSON.parse(v));
    }

    if (numberOfRuns === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${
          competitionName || defaultCompetitionName
        }/numberOfRuns`,
      );
      setNumberOfRuns(v === null ? defaultNumberOfRuns : JSON.parse(v));
    }

    if (resetRepetitionsFrequency === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${
          competitionName || defaultCompetitionName
        }/resetRepetitionsFrequency`,
      );
      setResetRepetitionsFrequency(
        v === null ? defaultResetRepetitionsFrequency : JSON.parse(v),
      );
    }
  });

  useEffect(() => {
    if (
      competitionName === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;
    localStorage.setItem(
      "simulator/competitionName",
      JSON.stringify(competitionName),
    );
  }, [competitionName]);

  useEffect(() => {
    if (
      numberOfRuns === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;
    localStorage.setItem(
      `simulator/competition/${
        competitionName || defaultCompetitionName
      }/numberOfRuns`,
      JSON.stringify(numberOfRuns),
    );
  }, [numberOfRuns, competitionName]);

  useEffect(() => {
    if (
      resetRepetitionsFrequency === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;
    localStorage.setItem(
      `simulator/competition/${
        competitionName || defaultCompetitionName
      }/resetRepetitionsFrequency`,
      JSON.stringify(resetRepetitionsFrequency),
    );
  }, [resetRepetitionsFrequency, competitionName]);

  const {
    data: uniqueTricks,
    error,
    isLoading,
  } = useSWR<UniqueTrick[], Error>(`${API_URL}/tricks/unique/`);

  if (isLoading || competitionName === undefined || numberOfRuns === undefined)
    return <FetchLoading />;
  if (error) return <FetchError />;
  if (!uniqueTricks) return <h2>No Tricks found</h2>;

  return (
    <>
      <h1>Runs Simulator</h1>
      <button
        onClick={(e) => {
          e.preventDefault();
        }}
      >
        clear all
      </button>
      <div className="flew w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner lg:col-span-full">
        <article className="grid grid-cols-12 justify-center">
          {finalScore !== undefined && (
            <>
              <h2 className="col-span-10 col-start-2 bg-awt-dark-700 py-3 text-white">
                Final Score: {finalScore.toFixed(3)}
              </h2>
              <hr className="coll-span-full col-start-1 py-2" />
            </>
          )}

          <h4 className="col-span-4 col-start-1">Current Simulation</h4>
          <h4 className="col-span-4">Number of Runs</h4>
          <h4 className="col-span-4">Reset Repetitions</h4>
          <h6 className="col-span-4 col-start-1">{competitionName}</h6>
          <h6 className="col-span-4">
            <select
              value={numberOfRuns}
              onChange={(e) => setNumberOfRuns(parseInt(e.target.value))}
            >
              {Array(maxNumberOfRuns)
                .fill(false)
                .map((_, i) => (
                  <option key={i} value={i + 1}>
                    {i + 1}
                  </option>
                ))}
            </select>
          </h6>
          <h6 className="col-span-4">
            <select
              value={resetRepetitionsFrequency}
              onChange={(e) =>
                setResetRepetitionsFrequency(parseInt(e.target.value))
              }
            >
              <option value="0">never</option>
              <option value="1">at each run</option>
              <option value="2">at run 3 and 5</option>
              <option value="4">at run 4</option>
            </select>
          </h6>
        </article>
      </div>
      <div className="mt4 flex w-full items-start justify-center gap-5 portrait:flex-col">
        {[...Array(numberOfRuns)].map((_, i) => {
          return (
            <SimulatorRun
              competitionName={competitionName}
              key={i}
              runIndex={i}
              uniqueTricks={uniqueTricks}
              setRun={setRun}
              results={results[i]}
            />
          );
        })}
      </div>

      {finalScore !== undefined && (
        <div className="flew w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner lg:col-span-full">
          <hr className="coll-span-full col-start-1 py-2" />
          <h2 className="col-span-10 col-start-2 bg-awt-dark-700 py-3 text-white">
            Final Score: {finalScore.toFixed(3)}
          </h2>
        </div>
      )}
    </>
  );
};

export default Tricks;
