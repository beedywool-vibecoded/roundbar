const require_index = require("../utils/index.js");
// lib/rules/no-arrow-functions-in-watch.ts
/**
* @author Sosuke Suzuki
*/
var no_arrow_functions_in_watch_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using arrow functions to define watcher",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-arrow-functions-in-watch.html"
		},
		fixable: null,
		schema: [],
		messages: { noArrowFunctionsInWatch: "You should not use an arrow function to define a watcher." }
	},
	create(context) {
		return require_index.default.executeOnVue(context, (obj) => {
			const watchNode = require_index.default.findProperty(obj, "watch");
			if (watchNode == null) return;
			const watchValue = watchNode.value;
			if (watchValue.type !== "ObjectExpression") return;
			for (const property of watchValue.properties) {
				if (property.type !== "Property") continue;
				for (const handler of require_index.default.iterateWatchHandlerValues(property)) if (handler.type === "ArrowFunctionExpression") context.report({
					node: handler,
					messageId: "noArrowFunctionsInWatch"
				});
			}
		});
	}
};
exports.default = no_arrow_functions_in_watch_default;
