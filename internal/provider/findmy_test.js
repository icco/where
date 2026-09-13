// Synthetic Accessibility fixtures. Run with node --test; no macOS UI access.
const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync(`${__dirname}/findmy.js`, "utf8");

function element(id, children = [], description = "", extra = {}) {
  return {
    attributes: { byName: (name) => ({ value: () => ({ AXIdentifier: id, ...extra })[name] }) },
    uiElements: () => children,
    description: () => description,
  };
}

function person(name, location = "", status = "") {
  return element("HomeElementCell", [
    element("HomeCellTitleLabel", [], name),
    element("HomeCellSubtitleLabel", [], location),
    element("HomeCellDetailLabel", [], status),
  ]);
}

function read(cells, { scrolling = false, permission = true, stuck = false } = {}) {
  let selected = false;
  let scroll = 0;
  const bar = element("", [], "", { AXRole: "AXScrollBar", AXOrientation: "AXVerticalOrientation" });
  Object.defineProperty(bar, "value", {
    get: () => () => scroll,
    set: (value) => { if (!stuck) scroll = value; },
  });
  const sidebar = element("FindMyListEntries", [...cells, ...(scrolling ? [bar] : [])]);
  const window = element("SceneWindow", [
    // Map labels must never be interpreted as people.
    person("Map annotation", "Not a person"),
    sidebar,
  ]);
  const process = {
    windows: [window],
    menuBars: [{ menuBarItems: { byName: (menu) => {
      assert.equal(menu, "View");
      return { menus: [{ menuItems: { byName: (item) => {
        assert.equal(item, "People");
        return { click: () => { selected = true; } };
      } } }] };
    } } }],
  };
  const context = vm.createContext({
    Application: (name) => name === "FindMy" ? { activate() {} } : {
      uiElementsEnabled: () => permission,
      processes: { byName: () => process },
    },
    delay() {},
  });
  const output = vm.runInContext(`${source}\nrun();`, context);
  assert.equal(selected, true);
  return JSON.parse(output);
}

test("Me-only sidebar is a successful empty sharing list", () => {
  assert.deepEqual(read([person("Me")]), { pages: [[]] });
});

test("read names, locations and freshness only from sidebar cells", () => {
  assert.deepEqual(read([person("Me"), person("Alex", "London, England", "Now")]), {
    pages: [[{ name: "Alex", location: "London, England", status: "Now" }]],
  });
});

test("read all scroll positions and detect failed scrolling", () => {
  const output = read([person("Alex")], { scrolling: true });
  assert.equal(output.pages.length, 21);
  assert.ok(output.pages.every((page) => page.length === 1));
  assert.throws(() => read([person("Alex")], { scrolling: true, stuck: true }), /scrolling failed/);
});

test("fail explicitly for absent permissions, unknown layouts and unnamed rows", () => {
  assert.throws(() => read([], { permission: false }), /Accessibility/);
  assert.throws(() => read([]), /No recognizable People rows/);
  assert.throws(() => read([person("")]), /Unnamed/);
});
