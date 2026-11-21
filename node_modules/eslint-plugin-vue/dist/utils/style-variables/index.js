const require_index = require("../index.js");
// lib/utils/style-variables/index.ts
var StyleVariablesContext = class {
	context;
	styles;
	references = [];
	vBinds = [];
	constructor(context, styles) {
		this.context = context;
		this.styles = styles;
		for (const style of styles) {
			const containers = style.children.filter((node) => node.type === "VExpressionContainer");
			for (const node of containers) {
				this.vBinds.push(node);
				for (const ref of node.references) if (ref.variable == null) this.references.push(ref);
			}
		}
	}
};
const cache = /* @__PURE__ */ new WeakMap();
/**
* Get the style vars context
*/
function getStyleVariablesContext(context) {
	const sourceCode = context.sourceCode;
	const df = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
	if (!df) return null;
	const styles = df.children.filter((e) => require_index.default.isVElement(e) && e.name === "style");
	if (styles.length === 0) return null;
	let ctx = cache.get(styles[0]);
	if (ctx) return ctx;
	ctx = new StyleVariablesContext(context, styles);
	cache.set(styles[0], ctx);
	return ctx;
}
exports.StyleVariablesContext = StyleVariablesContext;
exports.getStyleVariablesContext = getStyleVariablesContext;
