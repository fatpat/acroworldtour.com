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
      { website && (
        <div>
          <h5 className="inline-block py-2 pl-4 text-left capitalize">
            <a href={website} target="_blank">Official Website</a>
          </h5>
        </div>
      )}
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Type:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{type}</p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Location:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{location}</p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Pilots:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{numberOfPilots}</p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Start Date:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{startDate}</p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">End Date:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{endDate}</p>
      </div>
            {competition.judges.length > 0 && (
              <>
                <hr />
                <CompetitionJudges
                  judges={competition.judges}
                />
              </>
            )}
    </article>
  );
};

export default CompetitionSummary;
