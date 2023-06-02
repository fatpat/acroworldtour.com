import classNames from "classnames";
import { Fragment, useEffect, useState } from "react";

import { components } from "@/types";

import { ChevronIcon } from "../ui/icons";
import SeasonCategoryResults from "./seasonCategoryResults";
import SeasonSummary from "./seasonSummary";

interface Props {
  season: components["schemas"]["SeasonExport"];
}

const SeasonDetails = ({ season }: Props) => {
  const { name, results: resultCategories } = season;

  const [showCategories, setShowCategories] = useState(
    resultCategories.map(() => false)
  );

  const [hideExtra, setHideExtra] = useState(false);

  useEffect(() => {
    setHideExtra(showCategories.some((showCat) => showCat));
  }, [showCategories]);

  const toggleCategories = (index: number) => {
    const newShowCat = [...showCategories].fill(false);
    newShowCat[index] = !showCategories[index];

    setShowCategories(newShowCat);
  };

  return (
    <>
      <h2 className="mb-4">{name}</h2>
      {resultCategories.length === 0 && <h3>No results at the moment.</h3>}
      <div
        className={classNames(
          "mt-4 flex w-full items-start justify-center gap-4 portrait:flex-col"
        )}
      >
        <SeasonSummary
          season={season}
          className={classNames(
            "w-1/2 max-w-lg rounded-xl bg-awt-dark-50 px-2 py-2 pb-2 shadow-inner",
            "portrait:w-full",
            hideExtra && "landscape:hidden"
          )}
        />
        <section
          className={classNames(
            "flex w-full flex-grow flex-col gap-4 rounded-xl bg-awt-dark-50 py-2 shadow-inner",
            hideExtra ? "lg:col-span-full" : "lg:col-span-6 lg:col-start-4"
          )}
        >
          {resultCategories.map((resultCategory, catIndex) => {
            const category = resultCategory.type;
            const categoryResults = resultCategory.results.sort(
              (a, b) => b.score - a.score
            );

            return (
              <Fragment key={catIndex}>
                <button
                  title={`Click to open/close ${category} results`}
                  className={classNames(
                    "col-span-full flex cursor-pointer items-baseline justify-center"
                  )}
                  onClick={() => toggleCategories(catIndex)}
                  onKeyDown={({ key }) =>
                    key === "Enter" && toggleCategories(catIndex)
                  }
                >
                  <h3 className="capitalize">{`${category} results`}</h3>
                  <ChevronIcon
                    className={classNames(
                      "ml-2 h-3 w-auto",
                      !showCategories[catIndex] && "-rotate-90"
                    )}
                  />
                </button>
                {showCategories[catIndex] && (
                  <SeasonCategoryResults
                    results={categoryResults}
                    className="grid grid-cols-12 border-[1px]"
                  />
                )}
              </Fragment>
            );
          })}
        </section>
      </div>
    </>
  );
};

export default SeasonDetails;
