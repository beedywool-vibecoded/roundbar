const require_configs_index = require("./configs/index.js");
const require_plugin = require("./plugin.js");
// lib/index.ts
var lib_default = Object.assign(require_plugin.default, { configs: require_configs_index });
module.exports = lib_default;
