const require_index = require("../../utils/index.js");
// lib/rules/syntaxes/script-setup.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var script_setup_default = {
	supported: ">=2.7.0",
	createScriptVisitor(context) {
		const scriptSetup = require_index.default.getScriptSetupElement(context);
		if (!scriptSetup) return {};
		const reportNode = require_index.default.getAttribute(scriptSetup, "setup") || scriptSetup.startTag;
		return { Program() {
			context.report({
				node: reportNode,
				messageId: "forbiddenScriptSetup"
			});
		} };
	}
};
exports.default = script_setup_default;
