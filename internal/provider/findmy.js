// JavaScript for Automation, run by macOS's built-in osascript.
// Return only person labels from the People sidebar, never map annotations.
function run() {
  const systemEvents = Application("System Events");
  if (!systemEvents.uiElementsEnabled()) {
    throw new Error("Accessibility permission required");
  }
  Application("FindMy").activate();
  delay(1);
  const process = systemEvents.processes.byName("FindMy");

  function attribute(element, name) {
    try {
      return element.attributes.byName(name).value();
    } catch (_) {
      return null;
    }
  }

  function descendants(element) {
    const result = [];
    function visit(node, depth) {
      if (depth > 30) throw new Error("Unexpected Find My hierarchy depth");
      result.push(node);
      for (const child of node.uiElements()) visit(child, depth + 1);
    }
    visit(element, 0);
    return result;
  }

  function window() {
    if (process.windows.length === 0) throw new Error("Find My window closed");
    return process.windows[0];
  }

  for (let attempt = 0; process.windows.length === 0 && attempt < 30; attempt++) {
    delay(0.2);
  }
  window();
  process.frontmost = true;
  process.menuBars[0].menuBarItems.byName("View").menus[0].menuItems.byName("People").click();
  delay(1);

  function sidebar() {
    const list = descendants(window()).find(
      (element) => attribute(element, "AXIdentifier") === "FindMyListEntries",
    );
    if (!list) throw new Error("Cannot identify People sidebar");
    return list;
  }

  function page() {
    const elements = descendants(sidebar());
    const cells = elements.filter(
      (element) => attribute(element, "AXIdentifier") === "HomeElementCell",
    );
    const rows = [];
    let recognized = 0;
    for (const cell of cells) {
      const labels = {};
      for (const element of descendants(cell)) {
        const id = attribute(element, "AXIdentifier");
        if (id && id.indexOf("HomeCell") === 0) {
          labels[id] = element.description();
        }
      }
      const name = labels.HomeCellTitleLabel;
      if (!name) throw new Error("Unnamed Find My People row");
      recognized++;
      if (name === "Me") continue;
      rows.push({
        name: name,
        location: labels.HomeCellSubtitleLabel || "",
        status: labels.HomeCellDetailLabel || "",
      });
    }
    // A recognized Me row is a valid list with no shared people. A signed-out
    // screen or unknown layout must not be mistaken for that empty list.
    if (recognized === 0) throw new Error("No recognizable People rows");
    return rows;
  }

  // Wait for the People view to populate, rather than assuming one fixed delay.
  let initial;
  for (let attempt = 0; attempt < 30; attempt++) {
    delay(0.2);
    try {
      initial = page();
      break;
    } catch (error) {
      if (attempt === 29) throw error;
    }
  }
  const bars = descendants(sidebar()).filter(
    (element) => attribute(element, "AXRole") === "AXScrollBar" &&
      attribute(element, "AXOrientation") === "AXVerticalOrientation",
  );
  if (bars.length === 0) return JSON.stringify({ pages: [initial] });
  if (bars.length !== 1) throw new Error("Ambiguous People scroll bar");

  const bar = bars[0];
  const pages = [];
  for (let step = 0; step <= 20; step++) {
    const target = step / 20;
    bar.value = target;
    delay(0.2);
    if (Math.abs(Number(bar.value()) - target) > 0.02) {
      throw new Error("People scrolling failed");
    }
    // Go checks that adjacent pages overlap before accepting the combined list.
    pages.push(page());
  }
  return JSON.stringify({ pages: pages });
}
