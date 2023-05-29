import Image from "next/image";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

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
  } = competition;

  return (
    <div className={className}>
      <h3 className="mb-4">Summary</h3>
        {image && (
          <Image
            src={image}
            alt="Competition Image"
            width={512}
            height={0}
            className="my-2 h-auto w-full rounded-xl"
          />
        )}
        <p>{`Type: ${capitalise(type)}`}</p>
        <p>{`Location: ${location}`}</p>
        <p>{`Pilots: ${numberOfPilots}`}</p>
        <p>{`Start Date: ${startDate}`}</p>
        <p>{`End Date: ${endDate}`}</p>
    </div>
  );
};

export default CompetitionSummary;
