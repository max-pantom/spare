const availableRoot = document.querySelector("#available-jobs");
const plannedRoot = document.querySelector("#planned-jobs");
const template = document.querySelector("#job-card-template");

loadCatalog();

async function loadCatalog() {
  try {
    const response = await fetch("./catalog.json", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(`Catalog returned HTTP ${response.status}`);
    }
    const catalog = await response.json();
    const available = catalog.jobs.filter((job) => job.status === "available");
    const planned = catalog.jobs.filter((job) => job.status === "planned");
    availableRoot.replaceChildren(...available.map(createCard));
    plannedRoot.replaceChildren(...planned.map(createCard));
  } catch (error) {
    availableRoot.innerHTML =
      '<p class="loading" role="alert">Unable to load jobs. Refresh this page and try again.</p>';
    plannedRoot.replaceChildren();
    console.error(error);
  }
}

function createCard(job) {
  const card = template.content.firstElementChild.cloneNode(true);
  card.dataset.job = job.id;
  card.dataset.status = job.status;

  const icon = card.querySelector("img");
  icon.src = job.icon;

  card.querySelector(".job-status").textContent =
    job.status === "available" ? "Available" : `Wave ${job.wave}`;
  card.querySelector("h3").textContent = job.name;
  card.querySelector(".job-description").textContent = job.description;

  const features = card.querySelector(".feature-list");
  for (const feature of job.features) {
    const item = document.createElement("li");
    item.textContent = feature;
    features.append(item);
  }

  const metadata = card.querySelector(".job-meta");
  const version = document.createElement("span");
  version.textContent = job.version ? `Version ${job.version}` : "Planned";
  const trust = document.createElement("span");
  trust.textContent =
    job.status === "available" ? "Signed by Spare" : "Not downloadable yet";
  metadata.append(version, trust);

  const download = card.querySelector(".download-button");
  if (job.status === "available") {
    download.href = job.download;
    download.download = "";
    download.textContent = `Download ${job.name}`;
    download.setAttribute(
      "aria-label",
      `Download ${job.name} ${job.version} for Spare`,
    );
  } else {
    download.removeAttribute("href");
    download.textContent = "Planned";
  }
  return card;
}
