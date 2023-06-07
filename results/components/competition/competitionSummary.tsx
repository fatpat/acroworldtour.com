import Image from "next/image";

import { components } from "@/types";

import CompetitionJudges from "./competitionJudges";

interface Props {
  competition: components["schemas"]["CompetitionPublicExportWithResults"];
  className?: string;
}

const CompetitionSummary = ({ competition, className }: Props) => {
  const {
    end_date: endDate,
    image,
    location,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    start_date: startDate,
    type,
    website,
  } = competition;

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
        <p className="inline-block py-2 pl-4 text-left capitalize">
          {startDate} - {endDate}
        </p>
      </div>
      {competition.judges.length > 0 && (
        <>
          <hr />
          <CompetitionJudges judges={competition.judges} className="pt-4 " />
        </>
      )}
    </article>
  );
};

export default CompetitionSummary;
