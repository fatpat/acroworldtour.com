import classNames from "classnames";
import Link from "next/link";
import { useState } from "react";

import { components } from "@/types";

import JudgeCard from "../judgeCard";
import { ChevronIcon } from "../ui/icons";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
}

const CompetitionDetails = ({ competition }: Props) => {
  const {
    code,
    end_date: endDate,
    image,
    judges,
    location,
    name,
    number_of_pilots: numberOfPilots,
    pilots,
    results,
    start_date: startDate,
    state,
    type,
  } = competition;

  const overallResults = results.overall_results;
  const runsResults = results.runs_results;

  const [showOverall, setShowOverall] = useState(false);
  const [showRun, setShowRun] = useState(runsResults.map(() => true));

  const toggleRunVisibility = (index: number) => {
    const newShowRuns = [...showRun];
    newShowRuns[index] = !newShowRuns[index];
    setShowRun(newShowRuns);
  };

  return (
    <div
      style={{ backgroundImage: `url('${image}')` }}
      className={classNames(
        "bg-white/95 bg-cover bg-center bg-no-repeat bg-blend-overlay",
        "flex min-h-screen w-full flex-col items-center"
      )}
    >
      <h2>{name}</h2>
      <div
        className={classNames(
          "mt-4 flex w-full flex-col justify-evenly gap-6 px-4",
          "lg:flex-row "
        )}
      >
        <section
          className={classNames("flex min-w-max flex-col bg-green-200/10 p-4")}
        >
          <h3>Details</h3>
          <p>{`Type: ${type}`}</p>
          <p>{`Location: ${location}`}</p>
          <p>{`Pilots: ${numberOfPilots}`}</p>
          <p>{`Start Date: ${startDate}`}</p>
          <p>{`End Date: ${endDate}`}</p>
        </section>
        <section className={classNames("flex flex-col bg-red-200/30 p-4")}>
          <header
            className={classNames("flex cursor-pointer items-baseline")}
            onClick={() => setShowOverall(!showOverall)}
          >
            <h3>Overall Results</h3>
            <ChevronIcon
              className={classNames(
                "ml-2 h-3 w-auto",
                !showOverall && "-rotate-90"
              )}
            />
          </header>
          <p>
            Lorem ipsum dolor sit amet consectetur adipisicing elit. A ipsa
            eaque aliquid quibusdam, fuga ea laudantium quas atque aliquam
            inventore assumenda harum, molestias aperiam sit voluptas est earum
            quis qui. Quis qui quia quas praesentium vitae voluptatibus totam
            recusandae? Eos reprehenderit quasi aliquid enim est possimus
            consectetur aspernatur nobis ipsam, fugit excepturi exercitationem
            adipisci pariatur porro delectus similique repellendus esse! Nobis
            quam quia sit nostrum ex, unde asperiores ipsum deserunt velit.
            Aspernatur magnam odio ipsam dolorum porro ea aliquid numquam
            provident fugit qui quasi facere, inventore expedita consequuntur,
            iste rerum. Harum nobis dolore, at ullam vero nulla! Doloribus, ex
            hic! Nisi consectetur, asperiores iste, ad neque rerum tenetur
            distinctio dolor, non cum eius possimus quos laborum. Ab suscipit
            animi consectetur! Fugiat quis perspiciatis a. Animi mollitia
            repudiandae unde iste quibusdam praesentium incidunt hic ullam,
            maxime quasi dignissimos veniam dolore, maiores dolorem facilis,
            harum consectetur eaque tempora iure qui! Beatae, qui.
          </p>
          {showOverall && (
            <table className="w-11/12">
              <thead>
                <tr className="h-12">
                  <th className="text-left">Pilot</th>
                  <th className="text-right">Score</th>
                </tr>
              </thead>
              <tbody>
                {overallResults.map((result) => {
                  const { pilot, score, result_per_run } = result;
                  const roundedScore = Math.round(score * 100) / 100;
                  return (
                    <tr key={pilot!.name} className="h-12">
                      <td>{pilot!.name}</td>
                      <td className="text-right">{roundedScore}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}

          {runsResults.map((run, index) => {
            const runNumber = index + 1;

            return (
              <article
                key={run.results[0].pilot!.civlid}
                className={classNames("flex flex-col")}
              >
                <header
                  className={classNames("flex cursor-pointer items-baseline")}
                  onClick={() => toggleRunVisibility(index)}
                >
                  <h3>{`Run ${runNumber}`}</h3>
                  <ChevronIcon
                    className={classNames(
                      "ml-2 h-3 w-auto",
                      !showRun[index] && "-rotate-90"
                    )}
                  />
                </header>
                {showRun[index] &&
                  run.results.map((result) => {
                    const { pilot, tricks } = result;
                    const { name: pilotName, civlid: pilotId } = pilot!;

                    return <p key={pilotId}>{pilotName}</p>;
                  })}
              </article>
            );
          })}
        </section>

        <section
          className={classNames(
            "flex flex-col bg-blue-200/30 p-4"
          )}
        >
          <h3>Judges</h3>
          <article
            className={classNames(
              "mt-4 flex w-full flex-wrap justify-evenly gap-4"
            )}
          >
            {judges.map((judge) => (
              <JudgeCard key={judge.name} judge={judge} />
            ))}
          </article>
        </section>
      </div>
    </div>
  );
};

export default CompetitionDetails;
