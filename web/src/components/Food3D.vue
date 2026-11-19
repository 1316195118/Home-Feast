<script setup>
import { computed } from 'vue'

// 用 CSS 几何体捏出的立体食材（不是 emoji、不是图片）：
// 球形带蒂 / 锥形带缨 / 梭形鱼 / 层叠叶菜 / 蛋 / 块肉 / 蘑菇 / 瓶袋 …
// 每种给高光、暗面、投影，看起来是摆在层架上的小物件。
const props = defineProps({
  name: { type: String, required: true },
  tint: { type: String, default: '#eef3e8' },
  kind: { type: String, default: 'sphere' },
  color: { type: String, default: '#d94b3f' },
})
const style = computed(() => ({
  '--c': props.color,
  '--c-d': `color-mix(in srgb, ${props.color} 72%, #000)`,
  '--c-l': `color-mix(in srgb, ${props.color} 78%, #fff)`,
}))
</script>

<template>
  <span class="food3d" :style="style" :title="name">
    <!-- 圆果：番茄 / 苹果 / 橙 / 土豆 -->
    <span v-if="kind === 'sphere'" class="f-sphere">
      <span class="f-shine"></span><span class="f-stem"></span><span class="f-leaf"></span>
    </span>
    <!-- 锥根：胡萝卜 / 白萝卜 -->
    <span v-else-if="kind === 'cone'" class="f-cone">
      <span class="f-cone-leaf"></span><span class="f-cone-leaf b"></span>
    </span>
    <!-- 长条：黄瓜 / 茄子 / 玉米 / 莴笋 -->
    <span v-else-if="kind === 'long'" class="f-long">
      <span class="f-long-tip"></span><span class="f-shine"></span>
    </span>
    <!-- 叶菜：白菜 / 菠菜 / 生菜 -->
    <span v-else-if="kind === 'leafy'" class="f-leafy">
      <span class="f-leaf-a"></span><span class="f-leaf-b"></span><span class="f-leaf-c"></span>
    </span>
    <!-- 蛋类：鸡蛋 / 鸭蛋 -->
    <span v-else-if="kind === 'egg'" class="f-egg"><span class="f-shine"></span></span>
    <!-- 鱼：梭形 + 尾 -->
    <span v-else-if="kind === 'fish'" class="f-fish">
      <span class="f-tail"></span><span class="f-eye"></span><span class="f-fin"></span>
    </span>
    <!-- 虾：弯钩 -->
    <span v-else-if="kind === 'shrimp'" class="f-shrimp"><span class="f-eye"></span></span>
    <!-- 肉块：带纹理 -->
    <span v-else-if="kind === 'meat'" class="f-meat"><span class="f-fat"></span></span>
    <!-- 蘑菇 -->
    <span v-else-if="kind === 'mushroom'" class="f-mushroom"><span class="f-stem2"></span></span>
    <!-- 奶盒 / 袋 -->
    <span v-else-if="kind === 'carton'" class="f-carton"><span class="f-cap"></span></span>
    <!-- 碗 / 主食 -->
    <span v-else-if="kind === 'bowl'" class="f-bowl"><span class="f-foodtop"></span></span>
    <!-- 贝类：扇形带纹 -->
    <span v-else-if="kind === 'shell'" class="f-shell">
      <span class="f-shell-rib" v-for="i in 3" :key="i"></span>
    </span>
    <!-- 海带：飘带 -->
    <span v-else-if="kind === 'kelp'" class="f-kelp"><span></span></span>
    <!-- 青椒：圆椒带梗 -->
    <span v-else-if="kind === 'pepper'" class="f-pepper"><span class="f-pepper-stem"></span><span class="f-shine"></span></span>
    <!-- 西兰花：花球 + 茎 -->
    <span v-else-if="kind === 'broccoli'" class="f-broccoli"><span class="f-broc-stem"></span></span>
    <!-- 洋葱：圆润带浅纹 + 芽 -->
    <span v-else-if="kind === 'onion'" class="f-onion"><span class="f-onion-sprout"></span><span class="f-shine"></span></span>
    <!-- 南瓜：扁圆带棱 -->
    <span v-else-if="kind === 'pumpkin'" class="f-pumpkin"><span class="f-pumpkin-rib"></span><span class="f-pumpkin-stem"></span></span>
    <!-- 姜：不规则块 -->
    <span v-else-if="kind === 'ginger'" class="f-ginger"><span></span><span></span></span>
    <!-- 豆腐：白色方块 -->
    <span v-else-if="kind === 'tofu'" class="f-tofu"><span class="f-shine"></span></span>
    <!-- 腐竹：长条束 -->
    <span v-else-if="kind === 'stick'" class="f-stick"><span></span><span></span><span></span></span>
    <!-- 香蕉：弯月 -->
    <span v-else-if="kind === 'banana'" class="f-banana"><span class="f-banana-tip"></span></span>
    <!-- 菠萝：椭圆带叶冠 -->
    <span v-else-if="kind === 'pineapple'" class="f-pineapple">
      <span class="f-pine-leaf a"></span><span class="f-pine-leaf b"></span><span class="f-pine-leaf c"></span>
    </span>
    <!-- 豆粒 -->
    <span v-else-if="kind === 'beans'" class="f-beans">
      <span></span><span></span><span></span><span></span>
    </span>
    <!-- 默认块 -->
    <span v-else class="f-cube"><span class="f-shine"></span></span>
  </span>
</template>
