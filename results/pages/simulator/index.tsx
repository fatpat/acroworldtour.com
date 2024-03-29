import { IconButton } from "@mui/material";
import MenuItem from "@mui/material/MenuItem";
import TextField from "@mui/material/TextField";
import { useEffect, useState } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import SimulatorRefreshButton from "@/components/simulator/simulatorRefreshButton";
import SimulatorRun from "@/components/simulator/simulatorRun";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { DeleteIcon } from "@/components/ui/icons";
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
  const defaultResetRepetitionsFrequency = 2;

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

  const refreshResults = () => {
    if (resetRepetitionsFrequency === undefined) return;
    if (numberOfRuns === undefined) return;

    fetch(
      `${API_URL}/simulate/competition/solo?reset_repetitions_frequency=${resetRepetitionsFrequency}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(
          runs.map((run, i) => {
            if (!run || i >= numberOfRuns)
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
        let score: number | undefined = undefined;
        setResults(
          json.map((r: Flight, i: number) => {
            if (!runs) return undefined;
            if (!runs[i]) return undefined;
            if (!runs[i].tricks) return undefined;
            if (!runs[i].tricks) return undefined;

            if (runs[i].tricks.filter((t: any) => t !== undefined).length > 0) {
              score = (score || 0) + (r.final_marks?.score || 0);
              return r;
            }
            return undefined;
          }),
        );
        setFinalScore(score);
      });
    });
  };

  const setRun = (i: number, run: any) => {
    if (i < 0 || i >= maxNumberOfRuns) return;
    setRuns((previousRuns) => {
      let r = [...previousRuns];
      r[i] = run;
      return r;
    });
  };

  const cleanCompetition = () => {
    Object.keys(localStorage).map((key) => {
      if (key.startsWith(`simulator/competition/${competitionName}/`)) {
        localStorage.removeItem(key);
      }
      window.location.reload();
    });
  };

  useEffect(() => {
    refreshResults();
  }, [resetRepetitionsFrequency]);

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

  if (isLoading) return <FetchLoading />;
  if (competitionName === undefined) return <FetchLoading />;
  if (numberOfRuns === undefined) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!uniqueTricks) return <h2>No Tricks found</h2>;

  return (
    <>
      <h1>
        Competition Simulator
        <IconButton
          onClick={() => {
            if (!confirm(`Reset competition ?`)) return;
            cleanCompetition();
          }}
        >
          <DeleteIcon />
        </IconButton>
      </h1>
      <div className="flew w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner lg:col-span-full">
        <article className="grid grid-cols-12 justify-center">
          <h4 className="col-span-3 col-start-1">Current Simulation</h4>
          <h4 className="col-span-3">Number of Runs</h4>
          <h4 className="col-span-3">Reset tricks repetitions</h4>
          <h4 className="col-span-3">
            <SimulatorRefreshButton refresh={() => refreshResults()} />
          </h4>
          <h6 className="col-span-3 col-start-1">{competitionName}</h6>
          <h6 className="col-span-3">
            <TextField
              select
              value={numberOfRuns}
              onChange={(e) => setNumberOfRuns(parseInt(e.target.value))}
            >
              {Array(maxNumberOfRuns)
                .fill(false)
                .map((_, i) => (
                  <MenuItem key={i} value={i + 1}>
                    {i + 1}
                  </MenuItem>
                ))}
            </TextField>
          </h6>
          <h6 className="col-span-3">
            <TextField
              select
              value={resetRepetitionsFrequency}
              onChange={(e) =>
                setResetRepetitionsFrequency(parseInt(e.target.value))
              }
            >
              <MenuItem value="0">never</MenuItem>
              <MenuItem value="1">every run</MenuItem>
              <MenuItem value="2">at runs 3 and 5</MenuItem>
              <MenuItem value="4">at run 5</MenuItem>
            </TextField>
          </h6>
          <button
            className="col-span-3 cursor-pointer rounded-xl bg-awt-dark-700 text-white hover:invert"
            onClick={() => refreshResults()}
          >
            {finalScore === undefined
              ? "Click to get result !"
              : `Final Score: ${finalScore?.toFixed(3)}`}
          </button>
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

      <div className="flew w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner lg:col-span-full">
        <article className="grid grid-cols-12 justify-center">
          <h2 className="col-span-2 col-start-3">
            <SimulatorRefreshButton refresh={() => refreshResults()} />
          </h2>
          <button
            className="col-span-4 cursor-pointer rounded-xl bg-awt-dark-700 py-3 text-white hover:invert"
            onClick={() => refreshResults()}
          >
            {finalScore === undefined
              ? "Click to get result !"
              : `Final Score: ${finalScore?.toFixed(3)}`}
          </button>
          <h2 className="col-span-2">
            <SimulatorRefreshButton refresh={() => refreshResults()} />
          </h2>
        </article>
      </div>
    </>
  );
};

export default Tricks;
