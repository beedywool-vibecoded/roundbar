const require_package = require("./package.json.js");
// lib/meta.ts
var meta_default = {
	name: require_package.name,
	version: require_package.version
};
exports.default = meta_default;
