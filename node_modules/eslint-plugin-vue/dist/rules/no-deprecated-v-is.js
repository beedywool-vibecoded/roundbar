const require_index = require("../utils/index.js");
const require_v_is = require("./syntaxes/v-is.js");
// lib/rules/no-deprecated-v-is.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_v_is_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow deprecated `v-is` directive (in Vue.js 3.1.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-v-is.html"
		},
		fixable: null,
		schema: [],
		messages: { forbiddenVIs: "`v-is` directive is deprecated." }
	},
	create(context) {
		const templateBodyVisitor = require_v_is.default.createTemplateBodyVisitor(context);
		return require_index.default.defineTemplateBodyVisitor(context, templateBodyVisitor);
	}
};
exports.default = no_deprecated_v_is_default;
