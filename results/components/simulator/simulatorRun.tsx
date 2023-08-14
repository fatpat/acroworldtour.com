import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import { useEffect, useState } from "react";

import SimulatorMarkSelect from "@/components/simulator/simulatorMarkSelect";
import { components } from "@/types";

type UniqueTrick = components["schemas"]["UniqueTrick"];
type Flight = components["schemas"]["Flight"];
type Trick = string | undefined | null;

interface Props {
  competitionName: string;
  runIndex: number;
  uniqueTricks: UniqueTrick[];
  setRun: any;
  results: Flight | undefined;
}

const SimulatorRun = ({
  competitionName,
  runIndex,
  uniqueTricks,
  setRun,
  results,
}: Props) => {
  const numberOfTricksPerRun = 10;
  const defaultTricks = Array(numberOfTricksPerRun).fill(undefined);
  const defaultTechnical = 5;
  const defaultChoreography = 5;
  const defaultLanding = 5;
  const emptyTrick = {
    name: "",
    acronym: "",
    technical_coefficient: 0,
    bonus: 0,
    bonus_types: [],
    base_trick: "",
    uniqueness: [],
  };

  const [tricks, setTricks] = useState<Trick[] | undefined>(undefined);
  const [technical, setTechnical] = useState<number | undefined>(undefined);
  const [choreography, setChoreography] = useState<number | undefined>(
    undefined,
  );
  const [landing, setLanding] = useState<number | undefined>(undefined);

  useEffect(() => {
    if (typeof window === "undefined" || !window.localStorage) return;

    if (tricks === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${competitionName}/run/${runIndex}/tricks`,
      );
      if (v === null) setTricks(defaultTricks);
      else {
        let _tricks = JSON.parse(v) as Trick[];
        setTricks(_tricks.map((t) => (t === null ? undefined : t)));
      }
    }

    if (technical === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${competitionName}/run/${runIndex}/technical`,
      );
      if (v === null) v = String(defaultTechnical);
      setTechnical(JSON.parse(v));
    }

    if (choreography === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${competitionName}/run/${runIndex}/choreography`,
      );
      if (v === null) v = String(defaultChoreography);
      setChoreography(JSON.parse(v));
    }

    if (landing === undefined) {
      let v = localStorage.getItem(
        `simulator/competition/${competitionName}/run/${runIndex}/landing`,
      );
      if (v === null) v = String(defaultLanding);
      setLanding(JSON.parse(v));
    }
  });

  useEffect(() => {
    if (
      tricks === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;
    localStorage.setItem(
      `simulator/competition/${competitionName}/run/${runIndex}/tricks`,
      JSON.stringify(tricks),
    );
  }, [tricks, competitionName, runIndex]);

  useEffect(() => {
    if (
      technical === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;

    //    if (technical < 0) { setTechnical(0); return }
    //    if (technical > 10) { setTechnical(10); return }

    localStorage.setItem(
      `simulator/competition/${competitionName}/run/${runIndex}/technical`,
      JSON.stringify(technical),
    );
  }, [technical, competitionName, runIndex]);

  useEffect(() => {
    if (
      choreography === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;

    //    if (choreography < 0) { setChoreography(0); return }
    //    if (choreography > 10) { setChoreography(10); return }

    localStorage.setItem(
      `simulator/competition/${competitionName}/run/${runIndex}/choreography`,
      JSON.stringify(choreography),
    );
  }, [choreography, competitionName, runIndex]);

  useEffect(() => {
    if (
      landing === undefined ||
      typeof window === "undefined" ||
      !window.localStorage
    )
      return;

    //    if (landing < 0) { setLanding(0); return }
    //    if (landing > 10) { setLanding(10); return }

    localStorage.setItem(
      `simulator/competition/${competitionName}/run/${runIndex}/landing`,
      JSON.stringify(landing),
    );
  }, [landing, competitionName, runIndex]);

  useEffect(() => {
    if (tricks === undefined) return;
    if (technical === undefined) return;
    if (choreography === undefined) return;
    if (landing === undefined) return;

    setRun(runIndex, {
      tricks: tricks,
      technical: Math.min(Math.max(technical, 0), 10),
      choreography: Math.min(Math.max(choreography, 0), 10),
      landing: Math.min(Math.max(landing, 0), 10),
    });
  }, [tricks, technical, choreography, landing, runIndex, setRun]);

  if (tricks === undefined) return <></>;

  return (
    <section className="flew w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner lg:col-span-full">
      <article className="grid grid-cols-12 justify-center">
        <h2 className="col-span-full">run {runIndex + 1}</h2>
        {[...Array(numberOfTricksPerRun).keys()].map((i) => (
          <Autocomplete
            key={`run${runIndex}_trick${i + 1}`}
            disablePortal
            options={uniqueTricks.concat([emptyTrick])}
            getOptionLabel={(t) =>
              t?.name !== "" ? `${t.name} - ${t.acronym}` : ""
            }
            isOptionEqualToValue={(a, b) => a.name === b.name}
            groupBy={(t) => t?.base_trick}
            renderInput={(params) => <TextField {...params} />}
            value={uniqueTricks.find((t) => t.name === tricks[i]) || emptyTrick}
            onChange={(e, v) => {
              let t = tricks.slice();
              t.splice(i, 1, v?.name);
              setTricks(t);
            }}
            className="col-span-full"
          />
        ))}

        <hr className="coll-span-full col-start-1 py-3" />

        <h5 className="col-span-4 col-start-1 bg-awt-dark-500 py-3 text-white">
          Technical
        </h5>
        <h5 className="col-span-4 bg-awt-dark-500 py-3 text-white">
          Choreography
        </h5>
        <h5 className="col-span-4 bg-awt-dark-500 py-3 text-white">Landing</h5>
        <SimulatorMarkSelect
          value={technical}
          defaultValue={defaultTechnical}
          setValue={setTechnical}
          className="col-span-4 col-start-1"
        />
        <SimulatorMarkSelect
          value={choreography}
          defaultValue={defaultChoreography}
          setValue={setChoreography}
          className="col-span-4"
        />
        <SimulatorMarkSelect
          value={landing}
          defaultValue={defaultLanding}
          setValue={setLanding}
          className="col-span-4"
        />
        {results && tricks.find((t) => t !== undefined) && (
          <>
            <hr className="coll-span-full col-start-1 py-3" />

            <h4 className="col-span-full bg-awt-dark-500 py-3 text-white">
              Simulated Score
            </h4>
            <h5 className="col-span-full bg-awt-dark-700 py-3 text-white">
              {results.final_marks?.score.toFixed(3)}
            </h5>

            <hr className="coll-span-full col-start-1 py-2" />

            <h4 className="col-span-4 col-start-1 bg-awt-dark-500 py-3 text-white">
              Technicity
            </h4>
            <h4 className="col-span-4 bg-awt-dark-500 py-3 text-white">
              Bonus
            </h4>
            <h4 className="col-span-4 bg-awt-dark-500 py-3 text-white">
              Malus
            </h4>
            <h6 className="col-span-4 col-start-1 py-3">
              {results.final_marks?.technicity.toFixed(3)}
            </h6>
            <h6 className="col-span-4 py-3">
              {results.final_marks?.bonus_percentage.toFixed(1)}%
            </h6>
            <h6 className="col-span-4 py-3">
              {results.final_marks?.malus.toFixed(1)}%
            </h6>

            <hr className="coll-span-full col-start-1 py-2" />

            <h4 className="col-span-full bg-awt-dark-500 py-3 text-white">
              Final marks
            </h4>
            <h5 className="col-span-3 py-3">Technicity</h5>
            <h5 className="col-span-3 py-3">Choreography</h5>
            <h5 className="col-span-3 py-3">Landing</h5>
            <h5 className="col-span-3 py-3">Bonus</h5>
            <h6 className="col-span-3 col-start-1 py-3">
              {results.final_marks?.technicity.toFixed(3)}
            </h6>
            <h6 className="col-span-3 py-3">
              {results.final_marks?.choreography.toFixed(3)}
            </h6>
            <h6 className="col-span-3 py-3">
              {results.final_marks?.landing.toFixed(3)}
            </h6>
            <h6 className="col-span-3 py-3">
              {results.final_marks?.bonus.toFixed(3)}
            </h6>

            {(results.final_marks?.warnings.length || 0) > 0 && (
              <>
                <hr className="coll-span-full col-start-1 py-2" />
                <h4 className="col-span-full col-start-1 bg-awt-dark-500 py-3 text-white">
                  Warnings
                </h4>
                {results.final_marks?.warnings.map((w) => (
                  <p key={w} className="col-span-1 col-span-full py-1">
                    {w}
                  </p>
                ))}
              </>
            )}

            {(results.final_marks?.notes?.length || 0) > 0 && (
              <>
                <hr className="coll-span-full col-start-1 py-2" />
                <h4 className="col-span-full col-start-1 bg-awt-dark-500 py-3 text-white">
                  Notes
                </h4>
                {results.final_marks?.notes?.map((w) => (
                  <p key={w} className="col-span-1 col-span-full py-1">
                    {w}
                  </p>
                ))}
              </>
            )}
          </>
        )}
      </article>
    </section>
  );
};

export default SimulatorRun;
