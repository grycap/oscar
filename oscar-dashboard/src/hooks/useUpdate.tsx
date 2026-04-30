import { useEffect, useRef } from "react";

/**
 *  Hook that runs an effect only after the first render
 * @param effect - The effect to run
 * @param deps - The dependencies to watch
 */
function useUpdate(effect: React.EffectCallback, deps: React.DependencyList) {
  const firstRender = useRef(true);

  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false;
      return;
    }
    return effect();
  }, deps);
}

export default useUpdate;
