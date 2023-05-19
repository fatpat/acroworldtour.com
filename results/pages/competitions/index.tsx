import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pageTitle = "Acro World Tour | Competitions";
const pageDescription =
  "Acro World Tour Competitions page. In this page you will find all the past, present, and scheduled competitions.";

const Competitions = () => {
  const { data: competitions, error } = useSWR<Competition[]>(
    `${API_URL}/competitions`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!competitions) return <FetchLoading />;

  const currentCompetitions = competitions.filter(
    (comp) => comp.state === "open"
  );
  const previousCompetitions = competitions.filter(
    (comp) => comp.state === "closed"
  );
  const scheduledCompetitions = competitions.filter(
    (comp) => comp.state === "init"
  );

  return (
    <>
      <section>
        {currentCompetitions.length ? (
          <>
            <h2>Ongoing Competitions</h2>
            <div>
              {currentCompetitions.map((competition) =>
                CompetitionCard({ competition })
              )}
            </div>
          </>
        ) : (
          <h2 className="opacity-30">No ongoing competition.</h2>
        )}
      </section>
      <section>
        {scheduledCompetitions.length ? (
          <>
            <h2>Scheduled Competitions.</h2>
            <div>
              {scheduledCompetitions.map((competition) =>
                CompetitionCard({ competition })
              )}
            </div>
          </>
        ) : (
          <h2 className="opacity-30">No scheduled competition.</h2>
        )}
      </section>
      <section>
        <h2>Previous Competitions</h2>
        <div>
          {previousCompetitions.map((competition) =>
            CompetitionCard({ competition })
          )}
        </div>
      </section>
    </>
  );
};

Competitions.pageTitle = pageTitle;
Competitions.pageDescription = pageDescription;

export default Competitions;
