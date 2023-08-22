import Image from "next/image";

import { components } from "@/types";

import CompetitionJudges from "./competitionJudges";
import CompetitionPilots from "./competitionPilots";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
  className?: string;
}

const CompetitionSummary = ({ competition, className }: Props) => {
  let {
    start_date,
    end_date,
    image,
    location,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    type,
    website,
    pilots,
  } = competition;

  const startDate = new Date(start_date).toLocaleString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
  const endDate = new Date(end_date).toLocaleString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  pilots.sort((a, b) => a.rank - b.rank);

  return (
    <article className={className}>
      {image && (
        <Image
          src={image}
          alt="Competition Image"
          width={512}
          height={0}
          className="my-2 h-auto w-full rounded-xl"
        />
      )}
      {website && (
        <div>
          <h5 className="inline-block py-2 pl-4 text-left capitalize">
            <a href={website} target="_blank">
              Official Website
            </a>
          </h5>
        </div>
      )}
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">
          Location:
        </h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">
          {location}
        </p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">
          {type === "solo"
            ? `${numberOfPilots} pilots`
            : `${numberOfTeams} teams`}
        </h5>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Dates:</h5>
        <p className="inline-block py-2 pl-4 text-left">
          {`${startDate} - ${endDate}`}
        </p>
      </div>
      {type === "solo" && pilots.length > 0 && (
        <>
          <hr className="mt-4" />
          <CompetitionPilots pilots={pilots} className="pt-4" />
        </>
      )}
      {competition.judges.length > 0 && (
        <>
          <hr className="mt-4" />
          <CompetitionJudges judges={competition.judges} className="pt-4" />
        </>
      )}
    </article>
  );
};

export default CompetitionSummary;
